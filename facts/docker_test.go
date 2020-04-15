// Copyright 2015-2019 Bleemeo
//
// bleemeo.com an infrastructure monitoring solution in the Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// nolint: scopelint
package facts

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/docker/docker/api/types"
)

// testdata could be generated by running "docker inspect $NAME > testdata/$FILENAME"
// Some file use older version of Docker, which could be done with:
// docker-machine create --virtualbox-boot2docker-url https://github.com/boot2docker/boot2docker/releases/download/v19.03.5/boot2docker.iso machine-name
//
// The following container are used
// docker run -d --name noport busybox sleep 99d
// docker run -d --name my_nginx -p 8080:80 nginx
// docker run -d --name my_redis redis
// docker run -d --name multiple-port -p 5672:5672 rabbitmq
// docker run -d --name multiple-port2 rabbitmq
// docker run -d --name non-standard-port -p 4242:4343 -p 1234:1234 rabbitmq
//
// Other container using docker-compose are used (see docker-compose.yaml in testdata folder)

func TestContainer_ListenAddresses(t *testing.T) {
	type fields struct {
		primaryAddress string
	}

	tests := []struct {
		jsonFileName string
		fields       fields
		want         []ListenAddress
	}{
		{
			jsonFileName: "busybox-v19.03.5.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{},
		},
		{
			jsonFileName: "redis-v19.03.5.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 6379},
			},
		},
		{
			jsonFileName: "nginx-v19.03.5.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 80},
			},
		},
		{
			jsonFileName: "rabbitmq-v19.03.5.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 5672},
			},
		},
		{
			jsonFileName: "rabbitmq2-v19.03.5.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 4369},
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 5671},
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 5672},
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 25672},
			},
		},
		{
			jsonFileName: "rabbitmq-non-standard-ports-v19.03.5.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 1234},
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 4343},
			},
		},
		{
			jsonFileName: "redis-v18.09.4.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 6379},
			},
		},
		{
			jsonFileName: "nginx-17.06.0-ce.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 80},
			},
		},
		{
			jsonFileName: "rabbitmq-v1.13.1.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 5672},
			},
		},
		{
			jsonFileName: "testdata_rabbitmqExposed_1.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 5671},
			},
		},
		{
			jsonFileName: "testdata_rabbitmqInternal_1.json",
			fields: fields{
				primaryAddress: "10.0.0.42",
			},
			want: []ListenAddress{
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 4369},
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 5671},
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 5672},
				{Address: "10.0.0.42", NetworkFamily: "tcp", Port: 25672},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.jsonFileName, func(t *testing.T) {
			data, err := ioutil.ReadFile(filepath.Join("testdata", tt.jsonFileName))
			if err != nil {
				t.Error(err)
			}

			var result []types.ContainerJSON

			err = json.Unmarshal(data, &result)
			if err != nil {
				t.Error(err)
			}

			c := Container{
				primaryAddress: tt.fields.primaryAddress,
				inspect:        result[0],
			}

			if got := c.ListenAddresses(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Container.ListenAddresses() = %v, want %v", got, tt.want)
			}
		})
	}
}
