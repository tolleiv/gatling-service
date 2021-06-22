package main

import (
	"errors"
	"fmt"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	log "github.com/sirupsen/logrus"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
)

// getGatlingConf loads gatling.conf.yaml for the current service
func getGatlingConf(myKeptn *keptnv2.Keptn, project string, stage string, service string) (*GatlingConf, error) {
	var err error

	confFile := path.Join(ResourcePrefix, ConfFilename)
	log.Infof("Loading %s for %s.%s.%s", confFile, project, stage, service)

	keptnResourceContent, err := myKeptn.GetKeptnResource(confFile)

	if err != nil {
		logMessage := fmt.Sprintf("error when trying to load %s file for service %s on stage %s or project-level %s: %s", confFile, service, stage, project, err.Error())
		return nil, errors.New(logMessage)
	}
	if len(keptnResourceContent) == 0 {
		// if no configuration file is available, this is not an error, as the service will proceed with the default workload
		log.Warnf("no %s found", confFile)
		return nil, nil
	}

	var gatlingConf *GatlingConf
	gatlingConf, err = parseGatlingConf(keptnResourceContent)
	if err != nil {
		logMessage := fmt.Sprintf("Couldn't parse %s file found for service %s in stage %s in project %s. Error: %s", confFile, service, stage, project, err.Error())
		return nil, errors.New(logMessage)
	}

	log.Infof("Successfully loaded %s with %d workloads", ConfFilename, len(gatlingConf.Workloads))

	return gatlingConf, nil
}

// getAllGatlingResources copy all service specific files to our local environment
func getAllGatlingResources(myKeptn *keptnv2.Keptn, project string, stage string, service string, tempDir string) (int, error) {
	resources, err := myKeptn.ResourceHandler.GetAllServiceResources(project, stage, service)

	if err != nil {
		log.Warnf("Error getting gatling files: %s", err.Error())
		return 0, err
	}

	downloaded := 0
	for _, resource := range resources {
		if strings.Contains(*resource.ResourceURI, "gatling/") {
			log.Infof("Found file: %s", *resource.ResourceURI)
			_, err := getKeptnResource(myKeptn, *resource.ResourceURI, tempDir)

			if err != nil {
				return 0, err
			}
			downloaded++
		}
	}

	return downloaded, nil
}

// getKeptnResource fetches a resource from Keptn config repo and stores it in a temp directory
func getKeptnResource(myKeptn *keptnv2.Keptn, resourceName string, tempDir string) (string, error) {
	requestedResourceContent, err := myKeptn.GetKeptnResource(resourceName)

	if err != nil {
		log.Warnf("Failed to fetch file: %s\n", err.Error())
		return "", err
	}

	// Cut away folders from the sourcePath (if there are any)
	sourcePathParts := strings.Split(resourceName, string(os.PathSeparator))
	fullPathParts := append([]string{tempDir}, sourcePathParts[2:]...)
	targetFileName := path.Join(fullPathParts...)
	targetDirname := path.Dir(targetFileName)

	err = os.MkdirAll(targetDirname, 700)
	if err != nil {
		log.Errorf("Failed to create tempfolder: %s\n", err.Error())
		return "", err
	}
	resourceFile, err := os.Create(targetFileName)
	defer resourceFile.Close()

	_, err = resourceFile.Write(requestedResourceContent)

	if err != nil {
		log.Errorf("Failed to create tempfile: %s\n", err.Error())
		return "", err
	}

	return targetFileName, nil
}

// getServiceURL returns the service URL that is either passed via the DeploymentURI* parameters or constructs one based on keptn naming structure
func getServiceURL(data *keptnv2.TestTriggeredEventData) (*url.URL, error) {
	if len(data.Deployment.DeploymentURIsLocal) > 0 && data.Deployment.DeploymentURIsLocal[0] != "" {
		return url.Parse(data.Deployment.DeploymentURIsLocal[0])

	} else if len(data.Deployment.DeploymentURIsPublic) > 0 && data.Deployment.DeploymentURIsPublic[0] != "" {
		return url.Parse(data.Deployment.DeploymentURIsPublic[0])
	}

	return nil, errors.New("no deployment URI included in event")
}

// borrowed from go-utils, remove when https://github.com/keptn/go-utils/pull/286 is merged and new go-utils version available
func ExecuteCommandWithEnv(command string, args []string, env []string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = append(cmd.Env, env...)
	log.Debugf("executing command %s %s", command, strings.Join(args, " "))

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Error executing command %s %s: %s\n%s", command, strings.Join(args, " "), err.Error(), string(out))
	}
	return string(out), nil
}
