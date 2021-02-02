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

package informationcollector

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/go-logr/logr"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/types"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/util"
)

// fileStatusCollector manages information that finding status of files.
type fileStatusCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// fileStatusCollectorEnabled indicates whether fileStatusCollector is enabled.
	fileStatusCollectorEnabled bool
}

// NewFileStatusCollector creates a new fileStatusCollector.
func NewFileStatusCollector(
	ctx context.Context,
	logger logr.Logger,
	fileStatusCollectorEnabled bool,
) types.DiagnosisProcessor {
	return &fileStatusCollector{
		Context:                    ctx,
		Logger:                     logger,
		fileStatusCollectorEnabled: fileStatusCollectorEnabled,
	}
}

// Handler handles http requests for file information.
func (fc *fileStatusCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !fc.fileStatusCollectorEnabled {
		http.Error(w, fmt.Sprintf("file status collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to read request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var diagnosis diagnosisv1.Diagnosis
		err = json.Unmarshal(body, &diagnosis)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to unmarshal request body into an diagnosis: %v", err), http.StatusNotAcceptable)
			return
		}

		// List all file paths in context.
		paths, err := util.ListFilePathsFromFilePathInformationContext(diagnosis, fc)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list file paths: %v", err), http.StatusInternalServerError)
			return
		}

		fileStatusLists := make([]types.FileStatusList, 0, len(paths))
		for _, path := range paths {
			fileInfo, err := os.Stat(path)
			if err != nil {
				fc.Error(err, "failed to retrieve information of file", "file", path)
				continue
			}
			absolutePath, err := filepath.Abs(path)
			if err != nil {
				fc.Error(err, "failed to get absolute path", "file", path)
				continue
			}

			// Set file status and continue if the file is not a directory.
			if !fileInfo.IsDir() {
				sys, ok := fileInfo.Sys().(*syscall.Stat_t)
				if !ok {
					fc.Error(err, "failed to get underlying data source of file", "file", path)
					continue
				}

				diskUsage, err := util.DiskUsage(absolutePath)
				if err != nil {
					fc.Error(err, "failed to get disk usage", "file", path)
					continue
				}
				fileStatusList := types.FileStatusList{
					FileStatus: types.FileStatus{
						UID:       sys.Uid,
						GID:       sys.Gid,
						Inode:     sys.Ino,
						Links:     sys.Nlink,
						Mode:      fileInfo.Mode().String(),
						ModTime:   fileInfo.ModTime(),
						DiskUsage: diskUsage,
						Path:      path,
					},
				}
				fileStatusLists = append(fileStatusLists, fileStatusList)
				continue
			} else {
				// Retrieves all file entries under the directory.
				entries, err := ioutil.ReadDir(absolutePath)
				if err != nil {
					fc.Error(err, "failed to get all file entries", "file", absolutePath)
					continue
				}

				sys, ok := fileInfo.Sys().(*syscall.Stat_t)
				if !ok {
					fc.Error(err, "failed to get underlying data source of file", "file", path)
					continue
				}

				// Set file status of the directory.
				dirDiskUsage, err := util.DiskUsage(absolutePath)
				if err != nil {
					fc.Error(err, "failed to get disk usage", "file", absolutePath)
					continue
				}
				sorted := types.FileStatusList{
					FileStatus: types.FileStatus{
						UID:       sys.Uid,
						GID:       sys.Gid,
						Inode:     sys.Ino,
						Links:     sys.Nlink,
						Mode:      fileInfo.Mode().String(),
						ModTime:   fileInfo.ModTime(),
						DiskUsage: dirDiskUsage,
						Path:      absolutePath,
					},
					FileStatuses: make([]types.FileStatus, 0, len(entries)),
				}

				// Set file status of all file under the specified directory.
				for _, entry := range entries {
					sys, ok := entry.Sys().(*syscall.Stat_t)
					if !ok {
						fc.Error(err, "failed to get underlying data source of file", "file", path)
						continue
					}

					entryAbsolutePath := filepath.Join(absolutePath, entry.Name())
					diskUsage, err := util.DiskUsage(entryAbsolutePath)
					if err != nil {
						fc.Error(err, "failed to get disk usage", "file", entryAbsolutePath)
						continue
					}
					fileStatus := types.FileStatus{
						UID:       sys.Uid,
						GID:       sys.Gid,
						Inode:     sys.Ino,
						Links:     sys.Nlink,
						Mode:      entry.Mode().String(),
						ModTime:   entry.ModTime(),
						DiskUsage: diskUsage,
						Path:      entryAbsolutePath,
					}
					sorted.FileStatuses = append(sorted.FileStatuses, fileStatus)
				}
				sort.Sort(sort.Reverse(sorted))
				fileStatusLists = append(fileStatusLists, sorted)
			}
		}

		// Set file status information result in status context.
		diagnosis, err = util.SetDiagnosisStatusContext(diagnosis, util.FileStatusInformationContextKey, fileStatusLists)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to set context field: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(diagnosis)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal diagnosis: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
