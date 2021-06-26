package main

import (
	"fmt"
	"github.com/iancoleman/strcase"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path"
)

// GatlingConf Configuration file type
type GatlingConf struct {
	SpecVersion string      `json:"spec_version" yaml:"spec_version"`
	Workloads   []*Workload `json:"workloads" yaml:"workloads"`
}

// Workload of Keptn stage
type Workload struct {
	TestStrategy string `json:"teststrategy" yaml:"teststrategy"`
	Simulation   string `json:"simulation" yaml:"simulation"`
}

// parseGatlingConf parses config file content and maps it to the GatlingConf struct
func parseGatlingConf(input []byte) (*GatlingConf, error) {
	gatlingconf := &GatlingConf{}

	err := yaml.Unmarshal(input, &gatlingconf)
	if err != nil {
		return nil, err
	}
	return gatlingconf, nil
}

// determineSimulationName maps the TestStrategy to a simulation name
func determineSimulationName(data *keptnv2.TestTriggeredEventData, conf *GatlingConf) string {
	var simulation = fmt.Sprintf("%sSimulation", strcase.ToCamel(data.Test.TestStrategy))
	if conf != nil {
		for _, workload := range conf.Workloads {
			if workload.TestStrategy == data.Test.TestStrategy {
				if workload.Simulation != "" {
					simulation = workload.Simulation
					break
				}
			}
		}
	}
	return simulation
}

// restoreDefaultConfFiles will copy the default gatling config files to the temp directory
// in case they're missing in the resource files
func restoreDefaultConfFiles(rootDir, tempDir string) error {
	targetConf := path.Join([]string{tempDir, "conf"}...)
	err := os.MkdirAll(targetConf, 0700)
	if err != nil {
		return err
	}
	for _, file := range []string{"logback.xml", "gatling.conf", "gatling-akka.conf"} {
		sourceConfFile := path.Join([]string{rootDir, "opt", "gatling", "conf", file}...)
		targetConfFile := path.Join([]string{targetConf, file}...)

		if _, err := os.Stat(targetConfFile); err == nil {
			// file exists
			continue
		}
		srcFile, err := os.Open(sourceConfFile)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		destFile, err := os.Create(targetConfFile) // creates if file doesn't exist
		if err != nil {
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			return err
		}
	}
	return nil
}

type GatlingExecutionHandler func(args []string, env []string) (string, error)

func ScriptGatlingExecutionHandler(args []string, env []string) (string, error) {
	return ExecuteCommandWithEnv("gatling.sh", args, env)
}
