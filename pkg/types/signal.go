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
	"syscall"
)

// SignalList contains details to send signals to processes.
type SignalList []Signal

// Signal contains details to send a signal to a process.
type Signal struct {
	PID    int            `json:"pid"`
	Signal syscall.Signal `json:"signal"`
}
