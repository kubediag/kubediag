package function

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

// Template read template.yaml within language templates folder.
// TODO: add image build configuration
type Template struct {
	Language string `yaml:"language, omitempty"`
}

// ParseAllTemplates parses all template.yaml files within language template folder.
func ParseAllTemplates(templates *[]Template) error {
	files, err := ioutil.ReadDir("./templates")
	if err != nil {
		return err
	}

	for _, f := range files {
		lang := f.Name()
		if templatePath, err := GetTemplatePath(lang); err == nil {
			templateYAMLPath := templatePath + "/template.yml"
			fileData, err := ioutil.ReadFile(templateYAMLPath)
			if err != nil {
				return err
			}

			template := Template{}
			err = yaml.Unmarshal(fileData, &template)
			if err != nil {
				return err
			}

			*templates = append(*templates, template)
		}
	}

	return nil
}

// GetTemplatePath returns path of the language template folder
func GetTemplatePath(lang string) (string, error) {
	path := "./templates/" + lang
	_, err := os.Stat(path)

	return path, err
}
