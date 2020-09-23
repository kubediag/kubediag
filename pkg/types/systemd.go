/*
Copyright 2020 The Kube Diagnoser Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"bytes"
	"io"
	"strings"
)

// Unit represents an unit, a job or the manager itself.
// See systemctl(1) linux manual page for more details:
//
// https://www.man7.org/linux/man-pages/man1/systemctl.1.html
type Unit struct {
	// Name is the name of an unit, a job or the manager itself.
	Name string `json:"name"`
	// Properties is the property list of an unit, a job or the manager itself.
	Properties []Property `json:"properties"`
}

// Property represents a property entry of unit, job or the manager itself.
type Property struct {
	// Name is the name of a property.
	Name string `json:"name"`
	// Value is the value of a property.
	Value string `json:"value"`
}

// ParseProperties parses a "systemctl show" output to a property slice.
func ParseProperties(buf *bytes.Buffer) ([]Property, error) {
	properties := make([]Property, 0)
	for {
		line, err := buf.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		line = strings.TrimSuffix(line, "\n")
		str := strings.SplitN(line, "=", 2)
		property := Property{
			Name:  str[0],
			Value: str[1],
		}
		properties = append(properties, property)
	}

	return properties, nil
}
