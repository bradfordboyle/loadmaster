package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	yaml "gopkg.in/yaml.v2"
)

type Resource struct {
	Name   string
	Type   string
	Source map[string]interface{}
}

type ResourceTypeSource struct {
	Repository string
	Tag        string
}

func (r ResourceTypeSource) String() string {
	if r.Tag != "" {
		return fmt.Sprintf("%s:%s", r.Repository, r.Tag)
	}

	return r.Repository
}

type ResourceType struct {
	Name   string
	Type   string
	Source ResourceTypeSource
}

func (r *ResourceType) Check(request Request) ([]map[string]interface{}, error) {
	image := r.Source.String()
	cmd := exec.Command("docker", "run", "--rm", "--interactive", image, "/opt/resource/check")
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

func (r *ResourceType) Get(name string, request Request) (map[string]interface{}, error) {
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
	image := r.Source.String()
	cmd := exec.Command("docker", "run", "--rm", "--interactive", "--volume", volumeMapping, image, "/opt/resource/in", containerResourcePath)
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

type PipelineConfig struct {
	Resources     []Resource     `yaml:"resources"`
	ResourceTypes []ResourceType `yaml:"resource_types"`
}

type Request struct {
	Source  map[string]interface{} `json:"source"`
	Version map[string]interface{} `json:"version"`
}

func LoadPipeline(reader io.Reader) (PipelineConfig, error) {
	yamlFile, err := ioutil.ReadAll(reader)
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

func ResourceTypeCache(resourceTypes []ResourceType) map[string]ResourceType {
	cache := make(map[string]ResourceType)
	for _, r := range resourceTypes {
		cache[r.Name] = r
	}

	// add concourse git
	if _, ok := cache["git"]; !ok {
		cache["git"] = ResourceType{
			Name: "git",
			Type: "docker-image",
			Source: ResourceTypeSource{
				Repository: "concourse/git-resource",
				Tag:        "latest",
			}}
	}

	return cache
}

const Usage = `Usage:
%s <pipeline-config>
`

type GetResources []string

func (g *GetResources) Set(resource string) error {
	*g = append(*g, resource)
	return nil
}

func (g *GetResources) String() string {
	return fmt.Sprint(*g)
}

func main() {

	var getResources GetResources
	flag.Var(&getResources, "i", "<resouce-name>")
	flag.Parse()

	sort.Strings(getResources)

	pipelineYaml := os.Stdin
	if len(flag.Args()) >= 1 {
		if yamlFile, err := os.Open(flag.Arg(0)); err == nil {
			pipelineYaml = yamlFile
		} else {
			log.Fatalf("Open: %v", err)
		}
	}

	pipeline, _ := LoadPipeline(pipelineYaml)
	resourceTypes := ResourceTypeCache(pipeline.ResourceTypes)

	for _, resource := range pipeline.Resources {
		idx := sort.SearchStrings(getResources, resource.Name)
		if len(getResources) != 0 && (idx == len(getResources) || getResources[idx] != resource.Name) {
			continue
		}
		resourceType := resourceTypes[resource.Type]
		request := Request{
			Source:  resource.Source,
			Version: nil,
		}

		versions, err := resourceType.Check(request)
		if err != nil {
			log.Fatalf("Check: %v", err)
		}

		if len(versions) == 0 {
			continue
		}

		request.Version = versions[0]
		_, err = resourceType.Get(resource.Name, request)
		if err != nil {
			log.Fatalf("Get: %v", err)
		}
	}
}
