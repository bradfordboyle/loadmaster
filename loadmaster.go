package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

type Resource struct {
	Name   string
	Type   string
	Source map[string]interface{}
}

type PipelineConfig struct {
	Resources []Resource `yaml:"resources"`
}

type Request struct {
	Source  map[string]interface{} `json:"source"`
	Version map[string]interface{} `json:"version"`
}

func LoadPipeline(path string) (PipelineConfig, error) {
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("ReadFile: %v", err)
	}
	p := PipelineConfig{}
	err = yaml.Unmarshal(yamlFile, &p)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	return p, nil
}

func GetVersions(request Request) ([]map[string]interface{}, error) {
	cmd := exec.Command("docker", "run", "--rm", "--interactive", "frodenas/gcs-resource", "/opt/resource/check")
	b, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	cmd.Stdin = bytes.NewBuffer(b)
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	var versions []map[string]interface{}
	json.Unmarshal(out.Bytes(), &versions)

	return versions, nil
}

func GetResource(name string, request Request) (map[string]interface{}, error) {
	_, err := os.Stat(name)
	if os.IsNotExist(err) {
		errDir := os.Mkdir(name, 0755)
		if errDir != nil {
			return nil, errDir
		}
	}
	hostResourcePath, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	containerResourcePath := fmt.Sprintf("/srv/%s", name)
	volumeMapping := hostResourcePath + ":" + containerResourcePath
	cmd := exec.Command("docker", "run", "--rm", "--interactive", "--volume", volumeMapping, "frodenas/gcs-resource", "/opt/resource/in", containerResourcePath)
	b, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	cmd.Stdin = bytes.NewBuffer(b)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	var metadata map[string]interface{}
	json.Unmarshal(out.Bytes(), &metadata)

	return metadata, nil
}

const Usage = `Usage:
%s <pipeline-config>
`

func main() {
	if len(os.Args) != 2 {
		fmt.Printf(Usage, os.Args[0])
		os.Exit(-1)
	}
	pipeline, _ := LoadPipeline(os.Args[1])

	for _, resource := range pipeline.Resources {
		if resource.Type != "gcs" {
			continue
		}
		request := Request{
			Source:  resource.Source,
			Version: nil,
		}

		versions, err := GetVersions(request)
		if err != nil {
			log.Fatalf("GetVersion: %v", err)
		}

		if len(versions) == 0 {
			continue
		}

		request.Version = versions[0]
		metadata, err := GetResource(resource.Name, request)
		if err != nil {
			log.Fatalf("GetResource: %v", err)
		}

		fmt.Printf("%v\n", metadata)

	}
}
