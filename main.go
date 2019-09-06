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

package main

import (
	"agentgo/agent"
	"flag"
	"fmt"
	"os"

	_ "net/http/pprof"
)

//nolint: gochecknoglobals
var (
	runAsRoot = flag.Bool("yes-run-as-root", false, "Allows Bleemeo agent to run as root")
)

func main() {
	flag.Parse()
	if os.Getuid() == 0 && !*runAsRoot {
		fmt.Println("Error: trying to run Bleemeo agent as root without \"--yes-run-as-root\" option.")
		fmt.Println("If Bleemeo agent is installed using standard method, start it with:")
		fmt.Println("    service bleemeo-agent start")
		fmt.Println("")
		os.Exit(1)
	}
	agent.Run()
}
