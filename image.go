package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func PullDockerImage(ctx context.Context, imageName string) {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Println(err, "unable to intialize docker client")
	}
	defer cli.Close()
	imagePullResp, err := cli.ImagePull(
		ctx,
		imageName,
		types.ImagePullOptions{RegistryAuth: GetRegistryAuth()})
	if err != nil {
		log.Println(err, "unable to pull image")
	}
	defer imagePullResp.Close()

	scanner := bufio.NewScanner(imagePullResp)
	for scanner.Scan() {
		output := strings.Replace(scanner.Text(), "u003e", ">", -1)
		log.Println(output)
	}
	err = scanner.Err()
	if err != nil {
		log.Println(err, "error in scanning")
	}

}

func BuildDockerImage(ctx context.Context, dockerFile string) string {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Println(err, "unable to intialize docker client")
	}
	defer cli.Close()
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	if dockerFile == "" {
		dockerFile = "Dockerfile"
	}
	dockerFileReader, err := os.Open(dockerFile)
	if err != nil {
		log.Println(err, " :unable to open Dockerfile, ", dockerFile)
	}
	readDockerFile, err := ioutil.ReadAll(dockerFileReader)
	if err != nil {
		log.Println(err, " :unable to read dockerfile")
	}

	tarHeader := &tar.Header{
		Name: dockerFile,
		Size: int64(len(readDockerFile)),
	}
	err = tw.WriteHeader(tarHeader)
	if err != nil {
		log.Println(err, " :unable to write tar header")
	}
	_, err = tw.Write(readDockerFile)
	if err != nil {
		log.Println(err, " :unable to write tar body")
	}
	dockerFileTarReader := bytes.NewReader(buf.Bytes())
	imageName := "meli_" + strings.ToLower(dockerFile)
	imageBuildResponse, err := cli.ImageBuild(
		ctx,
		dockerFileTarReader,
		types.ImageBuildOptions{
			//PullParent:     true,
			//Squash:     true, currently only supported in experimenta mode
			Tags:           []string{imageName},
			Remove:         true, //remove intermediary containers after build
			ForceRemove:    true,
			SuppressOutput: false,
			Dockerfile:     dockerFile,
			Context:        dockerFileTarReader})
	if err != nil {
		log.Println(err, " :unable to build docker image")
	}
	defer imageBuildResponse.Body.Close()

	scanner := bufio.NewScanner(imageBuildResponse.Body)
	for scanner.Scan() {
		output := strings.Replace(scanner.Text(), "u003e", ">", -1)
		log.Println(output)
	}
	err = scanner.Err()
	if err != nil {
		log.Println(err, "error in scanning")
	}

	return imageName

}

func GetRegistryAuth() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err, "unable to find current user")
	}
	// TODO: the config can be in many places
	// try to find them and use them; https://github.com/docker/docker-py/blob/e9fab1432b974ceaa888b371e382dfcf2f6556e4/docker/auth.py#L269
	dockerAuth, err := ioutil.ReadFile(usr.HomeDir + "/.docker/config.json")
	if err != nil {
		log.Fatal(err, "unable to read docker auth file, ~/.docker/config.json")
	}

	type AuthData struct {
		Auths map[string]map[string]string `json:"auths,omitempty"`
	}
	data := &AuthData{}
	err = json.Unmarshal([]byte(dockerAuth), data)
	if err != nil {
		log.Fatal(err, "unable to unmarshal")
	}

	encodedAuth := data.Auths["https://index.docker.io/v1/"]["auth"]
	yourAuth, err := base64.StdEncoding.DecodeString(encodedAuth)
	if err != nil {
		log.Fatal(err, "unable to base64 decode")
	}
	userPass := fomatRegistryAuth(string(yourAuth))
	username := userPass[0]
	password := userPass[1]

	stringRegistryAuth := `{"username": "DOCKERUSERNAME", "password": "DOCKERPASSWORD", "email": null, "serveraddress": "https://index.docker.io/v1/"}`

	stringRegistryAuth = strings.Replace(stringRegistryAuth, "DOCKERUSERNAME", username, 1)
	stringRegistryAuth = strings.Replace(stringRegistryAuth, "DOCKERPASSWORD", password, 1)

	RegistryAuth := base64.URLEncoding.EncodeToString([]byte(stringRegistryAuth))
	return RegistryAuth
}
