package function

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"
	archivex "github.com/jhoonb/archivex"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/util"
)

const (
	BuildFolderRootPath  string = "/image_build/"
	DefaultHandlerFolder string = "function"
)

// BuildFunctionImage build docker image from Kubediag Operation and built-in language runtime templates.
func BuildFunctionImage(cli dockerclient.Client, operation *diagnosisv1.Operation, lang string, imageBuildMessage *bytes.Buffer) error {
	functionName := operation.Name

	tempPath, err := GetBuildFolder(functionName, lang)
	if err != nil {
		return err
	}

	// Update function files and requirements in handler folder within temporary build folder.
	handlerFolderPath := tempPath + "/" + DefaultHandlerFolder
	for filename, code := range operation.Spec.Processor.Function.CodeSource {
		functionFilePath := filepath.Join(handlerFolderPath, filename)
		err := util.CreateFile(functionFilePath, code)
		if err != nil {
			return fmt.Errorf("failed to create function file: %s", functionFilePath)
		}
	}

	tarFile := BuildFolderRootPath + "/" + functionName + ".tar"
	PrepareBuildContext(tempPath, tarFile)
	// Remove temporary files after image building.
	defer util.RemoveFiles(tempPath, tarFile)

	buildContext, err := os.Open(tarFile)
	if err != nil {
		return err
	}
	defer buildContext.Close()

	imageName, tag := GetImageNameAndTag(operation)
	opts := NewImageBuildOptions(imageName+":"+tag, "Dockerfile")

	// Set default timeout for image building to 5 minutes.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	imageBuildResponse, err := cli.ImageBuild(ctx, buildContext, opts)
	if err != nil {
		return err
	}
	defer imageBuildResponse.Body.Close()

	// Read information returned by docker server after building an image.
	imageBuildMessage.ReadFrom(imageBuildResponse.Body)

	return err
}

// GetBuildFolder creates temporaty folder to build docker image with language template.
func GetBuildFolder(functionName string, lang string) (string, error) {
	tempPath := BuildFolderRootPath + functionName

	// Clear folder before build
	if err := os.RemoveAll(tempPath); err != nil {
		return tempPath, err
	}

	if err := os.MkdirAll(tempPath, os.ModePerm); err != nil {
		return tempPath, err
	}

	templatePath, err := GetTemplatePath(lang)
	if err != nil {
		return tempPath, err
	}

	err = util.CopyFiles(templatePath, tempPath)
	if err != nil {
		return tempPath, err
	}

	return tempPath, nil
}

// PrepareBuildContext creates the tar file and add the files and directories required to build image.
func PrepareBuildContext(filePath, tarFileName string) {
	tar := new(archivex.TarFile)
	tar.Create(tarFileName)
	tar.AddAll(filePath, false)
	tar.Close()
}

func NewImageBuildOptions(image string, dockerfile string) dockertypes.ImageBuildOptions {
	return dockertypes.ImageBuildOptions{
		Dockerfile: dockerfile,
		Tags:       []string{image},
		NoCache:    false,
	}
}

// ImageExists checks if docker image exists in the docker host.
func ImageExists(cli dockerclient.Client, imageName string, tag string) bool {
	exist := false
	_, _, err := cli.ImageInspectWithRaw(context.Background(), imageName+":"+tag)
	if err == nil {
		exist = true
	}
	return exist
}

//GetImageNameAndTag generate image name and tag from Kubediag Operation.
func GetImageNameAndTag(operation *diagnosisv1.Operation) (imageName, tag string) {
	imageName = "kubediag-" + operation.Name
	// Compute image tag based on source code
	tag = util.ComputeHash(operation.Spec.Processor.Function.CodeSource)
	return
}
