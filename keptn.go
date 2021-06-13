package main

import (
	"fmt"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
)

func getAllGatlingResources(myKeptn *keptnv2.Keptn, project string, stage string, service string, tempDir string) error {
	resources, err := myKeptn.ResourceHandler.GetAllServiceResources(project, stage, service)

	if err != nil {
		log.Printf("Error getting gatling files: %s", err.Error())
		return err
	}

	for _, resource := range resources {
		if strings.Contains(*resource.ResourceURI, "gatling/") {
			log.Printf("Found file: %s", *resource.ResourceURI)
			_, err := getKeptnResource(myKeptn, *resource.ResourceURI, tempDir)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

// getKeptnResource fetches a resource from Keptn config repo and stores it in a temp directory
func getKeptnResource(myKeptn *keptnv2.Keptn, resourceName string, tempDir string) (string, error) {
	requestedResourceContent, err := myKeptn.GetKeptnResource(resourceName)

	if err != nil {
		log.Printf("Failed to fetch file: %s\n", err.Error())
		return "", err
	}

	// Cut away folders from the sourcePath (if there are any)
	sourcePathParts := strings.Split(resourceName, string(os.PathSeparator))
	fullPathParts := append([]string{tempDir}, sourcePathParts[2:]...)
	targetFileName := path.Join(fullPathParts...)
	targetDirname := path.Dir(targetFileName)

	err = os.MkdirAll(targetDirname, 700)
	if err != nil {
		log.Printf("Failed to create tempfolder: %s\n", err.Error())
		return "", err
	}
	resourceFile, err := os.Create(targetFileName)
	defer resourceFile.Close()

	_, err = resourceFile.Write(requestedResourceContent)

	if err != nil {
		log.Printf("Failed to create tempfile: %s\n", err.Error())
		return "", err
	}

	return targetFileName, nil
}

// borrowed from go-utils, remove when https://github.com/keptn/go-utils/pull/286 is merged and new go-utils version available
func ExecuteCommandWithEnv(command string, args []string, env []string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Error executing command %s %s: %s\n%s", command, strings.Join(args, " "), err.Error(), string(out))
	}
	return string(out), nil
}
