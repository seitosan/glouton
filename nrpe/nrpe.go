package nrpe

import (
	"agentgo/logger"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math/big"
	"net"
	"sync"
	"time"
)

type reducedPacket struct {
	packetVersion int16
	packetType    int16
	resultCode    int16
	buffer        string
}

// Server is an NRPE server than use Callback for reply to queries
type Server struct {
	callback    callback
	bindAddress string
	enableTLS   bool
}

// New returns a NRPE server
// callback is the function responsible to generate the response for a given query.
func New(bindAddress string, enableTLS bool, callback callback) Server {
	return Server{
		callback:    callback,
		bindAddress: bindAddress,
		enableTLS:   enableTLS,
	}
}

type callback func(ctx context.Context, command string) (string, int16, error)

func handleConnection(ctx context.Context, c io.ReadWriteCloser, cb callback) {
	decodedRequest, err := decode(c)
	if err != nil {
		logger.V(1).Printf("Unable to decode NRPE packet: %v", err)
		c.Close()
		return
	}

	var answer reducedPacket
	answer.buffer, answer.resultCode, err = cb(ctx, decodedRequest.buffer)
	answer.packetVersion = decodedRequest.packetVersion
	if err != nil {
		logger.V(1).Printf("%d", err)
	}

	var encodedAnswer []byte
	if answer.packetVersion == 3 {
		encodedAnswer, err = encodeV3(answer)
	} else {
		encodedAnswer, err = encodeV2(answer, [2]byte{0x53, 0x51})
	}
	if err != nil {
		logger.V(1).Printf("%v", err)
		c.Close()
		return
	}
	_, err = c.Write(encodedAnswer)
	if err != nil {
		logger.V(1).Printf("%v", err)
	}
	c.Close()
}

func decode(r io.Reader) (reducedPacket, error) {
	packetHead := make([]byte, 16)
	_, err := r.Read(packetHead)
	if err != nil {
		return reducedPacket{}, err
	}
	var bufferlength int32
	var decodedPacket reducedPacket

	buf := bytes.NewReader(packetHead)
	err = binary.Read(buf, binary.BigEndian, &decodedPacket.packetVersion)
	if err != nil {
		err = fmt.Errorf("binary.Read failed for packet_version: %v", err)
		return decodedPacket, err
	}
	err = binary.Read(buf, binary.BigEndian, &decodedPacket.packetType)
	if err != nil {
		err = fmt.Errorf("binary.Read failed for packet_type: %v", err)
		return decodedPacket, err
	}
	var crc32value uint32
	err = binary.Read(buf, binary.BigEndian, &crc32value)
	if err != nil {
		err = fmt.Errorf("binary.Read failed for crc32value: %v", err)
		return decodedPacket, err
	}
	err = binary.Read(buf, binary.BigEndian, &decodedPacket.resultCode)
	if err != nil {
		err = fmt.Errorf("binary.Read failed for result_code: %v", err)
		return decodedPacket, err
	}

	if decodedPacket.packetVersion == 3 {
		var uselessvariable int16
		err = binary.Read(buf, binary.BigEndian, &uselessvariable)
		if err != nil {
			err = fmt.Errorf("binary.Read failed for alignment: %v", err)
			return decodedPacket, err
		}
		err = binary.Read(buf, binary.BigEndian, &bufferlength)
		if err != nil {
			err = fmt.Errorf("binary.Read failed for buffer_length: %v", err)
			return decodedPacket, err
		}
	}
	if decodedPacket.packetVersion == 2 {
		bufferlength = 1017
	}

	packetBuffer := make([]byte, bufferlength+3)
	_, err = r.Read(packetBuffer)
	if err != nil {
		return reducedPacket{}, err
	}
	//test value CRC32
	completePacket := make([]byte, 19+bufferlength)
	copy(completePacket[:16], packetHead)
	copy(completePacket[16:], packetBuffer)
	completePacket[4] = 0
	completePacket[5] = 0
	completePacket[6] = 0
	completePacket[7] = 0
	if crc32.ChecksumIEEE(completePacket) != crc32value {
		return decodedPacket, errors.New("wrong value for crc32")
	}

	i := bytes.IndexByte(packetBuffer, 0x0)
	if decodedPacket.packetVersion == 3 {
		packetBuffer = packetBuffer[:i]
		decodedPacket.buffer = string(packetBuffer)
	}
	if decodedPacket.packetVersion == 2 {
		packetBuffer = packetBuffer[:i]
		decodedPacket.buffer = string(packetHead[10:]) + string(packetBuffer)
	}

	return decodedPacket, nil
}

func encodeV2(decodedPacket reducedPacket, randBytes [2]byte) ([]byte, error) {
	decodedPacket.packetType = 2

	encodedPacket := make([]byte, 1036)
	encodedPacket[1] = 0x02 //version 2 encoding

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, &decodedPacket.packetType)
	if err != nil {
		err = fmt.Errorf("binary.Write failed for packet_type: %v", err)
		return encodedPacket, err
	}
	copy(encodedPacket[2:4], buf.Bytes())

	buf = new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, &decodedPacket.resultCode)
	if err != nil {
		err = fmt.Errorf("binary.Write failed for result_code: %v", err)
		return encodedPacket, err
	}
	copy(encodedPacket[8:10], buf.Bytes())

	copy(encodedPacket[10:10+len(decodedPacket.buffer)], []byte(decodedPacket.buffer))
	encodedPacket[1034] = randBytes[0] //random bytes encoding
	encodedPacket[1035] = randBytes[1]

	crc32Value := crc32.ChecksumIEEE(encodedPacket)
	buf = new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, &crc32Value)
	if err != nil {
		err = fmt.Errorf("binary.Write failed for crc32_value: %v", err)
		return encodedPacket, err
	}
	copy(encodedPacket[4:8], buf.Bytes())

	return encodedPacket, nil
}

func encodeV3(decodedPacket reducedPacket) ([]byte, error) {
	decodedPacket.packetType = 2
	bufferLength := int32(len(decodedPacket.buffer))
	encodedPacket := make([]byte, 19+len(decodedPacket.buffer))

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, &decodedPacket.packetVersion)
	if err != nil {
		err = fmt.Errorf("binary.Write failed for packet_version: %v", err)
		return encodedPacket, err
	}
	copy(encodedPacket[:2], buf.Bytes())

	buf = new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, &decodedPacket.packetType)
	if err != nil {
		err = fmt.Errorf("binary.Write failed for packet_type: %v", err)
		return encodedPacket, err
	}
	copy(encodedPacket[2:4], buf.Bytes())

	buf = new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, &bufferLength)
	if err != nil {
		err = fmt.Errorf("binary.Write failed for bufferLength: %v", err)
		return encodedPacket, err
	}
	copy(encodedPacket[12:16], buf.Bytes())

	buf = new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, &decodedPacket.resultCode)
	if err != nil {
		err = fmt.Errorf("binary.Write failed for resultCode: %v", err)
		return encodedPacket, err
	}
	copy(encodedPacket[8:10], buf.Bytes())

	copy(encodedPacket[16:16+len(decodedPacket.buffer)], []byte(decodedPacket.buffer))

	crc32Value := crc32.ChecksumIEEE(encodedPacket)
	buf = new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, &crc32Value)
	if err != nil {
		err = fmt.Errorf("binary.Write failed for crc32_value: %v", err)
		return encodedPacket, err
	}
	copy(encodedPacket[4:8], buf.Bytes())
	return encodedPacket, nil
}

// helper function to create a cert template with a serial number and other required fields
func certTemplate() (*x509.Certificate, error) {
	// generate a random serial number (a real cert authority would have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.New("failed to generate serial number: " + err.Error())
	}

	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"Bleemeo"}},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		IsCA:                  true,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	return &tmpl, nil
}
func createCert(template, parent *x509.Certificate, pub interface{}, parentPriv interface{}) (
	cert *x509.Certificate, certPEM []byte, err error) {

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return
	}
	// parse the resulting certificate so we can use it again
	cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return
	}
	// PEM encode the certificate (this is a standard TLS encoding)
	b := pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEM = pem.EncodeToMemory(&b)
	return
}
func generateCert() (*tls.Config, error) {
	// generate a new key-pair
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	rootCertTmpl, err := certTemplate()
	if err != nil {
		return nil, err
	}

	_, rootCertPEM, err := createCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		return nil, err
	}
	// PEM encode the private key
	rootKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rootKey),
	})

	// Create a TLS cert using the private key and certificate
	rootTLSCert, err := tls.X509KeyPair(rootCertPEM, rootKeyPEM)
	if err != nil {
		return nil, err
	}

	// Configure the server to present the certficate we created
	return &tls.Config{
		Certificates: []tls.Certificate{rootTLSCert},
	}, nil
}

// Run start a connection with a nrpe server
func (s Server) Run(ctx context.Context) error {
	tcpAdress, err := net.ResolveTCPAddr("tcp", s.bindAddress)
	if err != nil {
		return err
	}
	l, err := net.ListenTCP("tcp", tcpAdress)

	if err != nil {
		return err
	}
	defer l.Close()
	lWrap := net.Listener(l)
	if s.enableTLS {
		certificate, err := generateCert()
		if err != nil {
			return err
		}
		lWrap = tls.NewListener(l, certificate)
	}

	var wg sync.WaitGroup
	for {
		err = l.SetDeadline(time.Now().Add(time.Second))
		if err != nil {
			break
		}
		c, err := lWrap.Accept()
		if ctx.Err() != nil {
			break
		}
		if errNet, ok := err.(net.Error); ok && errNet.Timeout() {
			continue
		}
		if err != nil {
			logger.V(1).Printf("Nrpe accept failed: %v", err)
			continue
		}

		err = c.SetDeadline(time.Now().Add(time.Second * 10))
		if err != nil {
			logger.V(1).Printf("Nrpe: setDeadline on connection failed: %v", err)
			c.Close()
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			handleConnection(ctx, c, s.callback)
		}()
	}
	wg.Wait()
	return err
}
