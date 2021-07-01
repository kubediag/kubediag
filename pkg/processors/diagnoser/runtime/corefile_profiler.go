package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/go-logr/logr"
	"github.com/joewalnes/websocketd/libwebsocketd"

	v1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/executor"
	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/utils"
)

type CoreFileProfilerType string

const (
	ParameterKeyCoreFileProfilerExpirationSeconds = "param.diagnoser.runtime.core_file_profiler.expiration_seconds"
	ParameterKeyCoreFileProfilerPid               = "param.diagnoser.runtime.core_file_profiler.pid"
	ParameterKeyCoreFileProfilerType              = "param.diagnoser.runtime.core_file_profiler.type"
	ParameterKeyCoreFileProfilerFilepath          = "param.diagnoser.runtime.core_file_profiler.filepath"

	ContextKeyCoreFileProfilerResultEndpoint = "diagnoser.runtime.core_file_profiler.result.endpoint"

	l1CoreFileSubPath      = "corefile/"
	l2CoreFileSubPathOfPod = "kubernetes/"
)

var (
	// typeGCore means gcore method, such as 'gdb --pid 23412'
	typeGCore CoreFileProfilerType = "gcore"
	// typeCoreDump means coredump method, such as 'gdb -c /data/core-file/app.core'
	typeCoreDump CoreFileProfilerType = "coredump"
)

// coreDumpTemplate is a HTTP template of CoreDumpHTTPServer
var coreDumpTemplate = `<html>
  <head>
     <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
     <title>Core File Debugging</title>
  </head>
  <body>
     <table border="1">
        {{ range . }}
        <tr>
            <td><a href="/download?file={{ . }} ">{{ . }} </a></td>
            <td><a href="/gdb?file={{ . }} " target="view_window"> debug </a></td>
        </tr>
        {{ else }}
        {{ end}}
     </table>
  </body>
</html>`

// gCoreTemplate is a HTTP template of GcoreHTTPServer
var gCoreTemplate = `<html>
  <head>
     <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
     <title>Process Debugging</title>
  </head>
  <body>
	<table border="1">
      <tr>
      {{ range .Titles }}
        <td>{{ . }}</td>
      {{ end }}
      </tr>
      {{ range $key, $value := .Processes }}
        <tr>
          {{ range $pindex, $pivalue := $value }}
            {{ if eq $pindex 1  }}
              <td><a href="/gdb?pid={{ $pivalue }} ">{{ $pivalue }} </a></td>
            {{ else }}
              <td>{{ $pivalue }}</td>
            {{ end }}
          {{ end }}
        </tr>
	  {{ end }}
    </table>
  </body>
</html>`

// CoreFileConfig contains some configuration required by coreFileProfiler.
type CoreFileConfig struct {
	// ExpirationSeconds is the life time of CoreFileHTTPServer.
	ExpirationSeconds uint64 `json:"expirationSeconds"`
	// Type is type of coreFileProfiler.
	Type CoreFileProfilerType `json:"type"`
	// FilePath is a specified path of core file.
	// If it is empty, CoreDumpHTTPServer will search core file in a path composed of pod information.
	FilePath string `json:"filePath,omitempty"`
	// Pid is a specified pid.
	// If it is 0, GCoreHTTPServer will find all processes in the container of the pod.
	Pid int `json:"pid,omitempty"`
	// Processes is an array of processes in the container of the pod
	Processes container.ContainerTopOKBody `json:"processes,omitempty"`
}

// coreFileProfiler will manage pods' core dump file and provide a websocket to do gdb online
type coreFileProfiler struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// client is the API client that performs all operations against a docker server.
	client *client.Client
	// corefilePorfilerEnabled indicates whether coreFileProfiler is enabled.
	corefilePorfilerEnabled bool
	// podCoreFilePath is root path of corefiles
	podCoreFilePath string
}

// NewCoreFileProfiler creates a new coreFileProfiler.
func NewCoreFileProfiler(ctx context.Context, log logr.Logger, dockerEndpoint string, corefileProfilerEnabled bool, dataRoot string) (processors.Processor, error) {
	coreFilePath := path.Join(dataRoot, l1CoreFileSubPath)
	podCoreFilePath := path.Join(coreFilePath, l2CoreFileSubPathOfPod)
	if corefileProfilerEnabled {
		err := configureCoreDumpPattern(coreFilePath, podCoreFilePath)
		if err != nil {
			return nil, err
		}
	}
	cli, err := client.NewClientWithOpts(client.WithHost(dockerEndpoint))
	if err != nil {
		return nil, err
	}
	return &coreFileProfiler{
		Context:                 ctx,
		Logger:                  log,
		client:                  cli,
		corefilePorfilerEnabled: corefileProfilerEnabled,
		podCoreFilePath:         podCoreFilePath,
	}, nil
}

// Handler handles http requests for corefile profiler.
func (c *coreFileProfiler) Handler(w http.ResponseWriter, r *http.Request) {
	if !c.corefilePorfilerEnabled {
		http.Error(w, fmt.Sprintf("corefile profiler is not enabled"), http.StatusUnprocessableEntity)
		return
	}
	switch r.Method {
	case "POST":
		c.Info("handle POST request")
		// read request body and unmarshal into a CoreFileConfig
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			c.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		expirationSeconds, err := strconv.Atoi(contexts[ParameterKeyCoreFileProfilerExpirationSeconds])
		if err != nil {
			c.Error(err, "invalid expirationSeconds field")
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			return
		}

		pid := 0
		if pidParam, exist := contexts[ParameterKeyCoreFileProfilerPid]; exist {
			pid, err = strconv.Atoi(pidParam)
			if err != nil {
				c.Error(err, "invalid pid field")
				http.Error(w, err.Error(), http.StatusNotAcceptable)
				return
			}
		}

		config := &CoreFileConfig{
			ExpirationSeconds: uint64(expirationSeconds),
			Type:              CoreFileProfilerType(contexts[ParameterKeyCoreFileProfilerType]),
			FilePath:          contexts[ParameterKeyCoreFileProfilerFilepath],
			Pid:               pid,
		}

		podInfo := utils.GetPodInfoFromContext(contexts)

		c.V(4).Info("get pod info", "pod info", podInfo)

		err = c.completeConfig(config, podInfo)
		if err != nil {
			c.Error(err, "complete config failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		port, err := utils.GetAvailablePort()
		if err != nil {
			c.Error(err, "get available port failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		mux := http.NewServeMux()
		var server *http.Server
		switch config.Type {
		case typeGCore:
			server, err = c.buildGCoreHTTPServer(config, port, mux)
		case typeCoreDump:
			server, err = c.buildCoreDumpHTTPServer(config, port, mux)
		default:
			err = fmt.Errorf("unknown core type: %s", config.Type)
		}
		if err != nil {
			c.Logger.Error(err, "failed to build corefile http server")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			defer cancel()
			err := server.ListenAndServe()
			if err == http.ErrServerClosed {
				c.Info("core file http server closed")
			} else if err != nil {
				c.Error(err, "failed to start core file http server")
			}

		}()

		// Shutdown core file http server with expiration duration.
		go func() {
			select {
			// Wait for core file http server error.
			case <-ctx.Done():
				return
			// Wait for expiration.
			case <-time.After(time.Duration(config.ExpirationSeconds) * time.Second):
				err := server.Shutdown(ctx)
				if err != nil {
					c.Error(err, "failed to shutdown core file http server")
				}
			}
		}()

		result := make(map[string]string)
		result[ContextKeyCoreFileProfilerResultEndpoint] = fmt.Sprintf("http://%s:%d", contexts[executor.NodeTelemetryKey], port)
		data, err := json.Marshal(result)
		if err != nil {
			c.Error(err, "failed to marshal response body")
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// configureCoreDumpPattern will deploy a script into /usr/local/bin/ and pipe core data to this script.
func configureCoreDumpPattern(coreFilePath, podCoreFilePath string) error {
	patternShell := []byte(`| /usr/local/bin/core_file_naming.sh %P %t`)
	namingScript := `#!/bin/bash
pid=$1
timestamp=$2

root='` + coreFilePath + `'
podCorefilePath='` + podCoreFilePath + `'
ls ${podCorefilePath} || mkdir -p ${podCorefilePath}
docker_root=$(docker info 2>&1|grep "Docker Root Dir"  |awk '{print $NF}')

if [ "$docker_root"x == ""x ] ; then
        realfile="/${root}/${pid}_$(date -d @${timestamp} "+%Y%m%d-%H%M%S")"
        cat  /dev/stdin > $realfile
        exit
fi

containerinfo=$(fgrep -h  -r "\"Pid\":${pid},"  ${docker_root}/containers/*/config.v2.json)
if [ "$containerinfo"x == ""x ] ; then
	realfile="${root}/${pid}_$(date -d @${timestamp} "+%Y%m%d-%H%M%S")"
else
	kube_info=$(echo $containerinfo | python -c "import sys,json;data=json.loads(sys.stdin.read());kube_info=data['Config']['Labels']['io.kubernetes.pod.namespace'] + '/' + data['Config']['Labels']['io.kubernetes.pod.name'] + '/' + data['Config']['Labels']['io.kubernetes.container.name'] ; print kube_info")
	realfile="${podCorefilePath}/${kube_info}/${pid}_$(date -d @${timestamp} "+%Y%m%d-%H%M%S")"
	mkdir -p ${podCorefilePath}/${kube_info}/
fi
cat /dev/stdin > $realfile
`
	ioutil.WriteFile("/usr/local/bin/core_file_naming.sh", []byte(namingScript), 0777)
	return ioutil.WriteFile("/proc/sys/kernel/core_pattern", patternShell, 0644)
}

// completeConfig will complete CoreFileConfig by setting Processes or FilePath
func (c *coreFileProfiler) completeConfig(config *CoreFileConfig, podInfo v1.PodReference) error {
	if config.Type == typeCoreDump && config.FilePath == "" {
		config.FilePath = path.Join(c.podCoreFilePath, podInfo.Namespace, podInfo.Name, podInfo.Container)
	}
	if config.Type == typeGCore && config.Pid == 0 {
		c.client.NegotiateAPIVersion(c)
		containers, err := c.client.ContainerList(c, dockertypes.ContainerListOptions{})
		if err != nil {
			c.Error(err, "list container failed")
			return fmt.Errorf("list container failed, %s", err.Error())
		}

		var cid string
		for _, cont := range containers {
			c.V(5).Info("get list container",
				"ns", cont.Labels["io.kubernetes.pod.namespace"],
				"name", cont.Labels["io.kubernetes.pod.name"],
				"container-name", cont.Labels["io.kubernetes.container.name"],
				"state", cont.State)
			if cont.Labels["io.kubernetes.pod.namespace"] == podInfo.Namespace &&
				cont.Labels["io.kubernetes.pod.name"] == podInfo.Name &&
				cont.Labels["io.kubernetes.container.name"] == podInfo.Container &&
				cont.State == "running" {
				c.V(5).Info("get container", "container", cont)
				cid = cont.ID
				break
			}
		}
		if cid != "" {
			topResult, err := c.client.ContainerTop(c, cid, nil)
			if err != nil {
				c.Error(err, "top container failed")
				return fmt.Errorf("top container failed, %s", err.Error())
			}
			c.V(4).Info("top result", "titles", topResult.Titles)
			c.V(4).Info("top result", "processes", topResult.Processes)
			if len(topResult.Processes) == 0 {
				c.Error(err, "top container empty")
				return fmt.Errorf("top container emtpy, %s", err.Error())
			}
			config.Processes = topResult
		} else {
			err = fmt.Errorf("failed to get a matched running container, pod info: %+v", podInfo)
			c.Error(err, "")
			return err
		}
	}
	return nil
}

// buildGCoreHTTPServer will start a web server to list processes and support gdb online
func (c *coreFileProfiler) buildGCoreHTTPServer(config *CoreFileConfig, port int, serveMux *http.ServeMux) (*http.Server, error) {
	if config.Pid > 0 {
		serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			g := buildGDBWebsocketHandler([]string{fmt.Sprintf("--pid=%d", config.Pid)})
			g.ServeHTTP(w, r)
		})
	} else {
		tpl := template.Must(template.New("h").Parse(gCoreTemplate))
		serveMux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			tpl.Execute(writer, config.Processes)
		})
		serveMux.Handle("/gdb", &gdbProcessHandler{})
	}
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: serveMux,
	}, nil
}

// buildCoreDumpHTTPServer will build a HTTP server to provide:
// 1. download coredump file
// 2. gdb debug coredump file online
func (c *coreFileProfiler) buildCoreDumpHTTPServer(config *CoreFileConfig, port int, serveMux *http.ServeMux) (*http.Server, error) {
	var files []string
	rootFileInfo, err := os.Stat(config.FilePath)
	if err != nil {
		return nil, err
	}
	if rootFileInfo.IsDir() {
		files, err = getAllFile(config.FilePath)
		if err != nil {
			return nil, err
		}
	} else {
		files = []string{config.FilePath}
	}
	c.V(5).Info("get all corefiles of pod", "files", files)

	tpl := template.Must(template.New("h").Parse(coreDumpTemplate))
	serveMux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		tpl.Execute(writer, files)
	})

	gdbCoreDumpHandler := &gdbCoreDumpHandler{
		subHandler: buildGDBWebsocketHandler([]string{}),
	}
	serveMux.Handle("/gdb", gdbCoreDumpHandler)
	serveMux.Handle("/download", &downloadHandler{})
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: serveMux,
	}, nil
}

// downloadHandler is a handler to download core dump files
type downloadHandler struct{}

func (d *downloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("file")
	filePath = path.Join(filePath)
	fi, err := os.Stat(filePath)
	if err == nil {
		if fi.IsDir() {
			http.Error(w, fmt.Sprintf("get a dir"), http.StatusInternalServerError)
			return
		}
		http.ServeFile(w, r, filePath)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// gdbProcessHandler is a handler to execute 'gdb --pid' online
type gdbProcessHandler struct{}

func (g *gdbProcessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("pid")
	gdbHandler := buildGDBWebsocketHandler([]string{fmt.Sprintf("--pid=%s", pid)})
	gdbHandler.ServeHTTP(w, r)
}

// gdbCoreDumpHandler is a handler to execute 'gdb -c ' online
type gdbCoreDumpHandler struct {
	subHandler *libwebsocketd.WebsocketdServer
}

func (cdh *gdbCoreDumpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("file")
	cdh.subHandler.Config.CommandArgs = []string{
		"-c",
		filePath,
	}
	cdh.subHandler.ServeHTTP(w, r)
}

// getAllFile will recursively get all files in the specified directory.
func getAllFile(pathname string) ([]string, error) {
	result := []string{}
	rd, err := ioutil.ReadDir(pathname)
	for _, fi := range rd {
		dPath := path.Join(pathname, fi.Name())
		if fi.IsDir() {
			subres, err := getAllFile(dPath)
			if err == nil {
				result = append(result, subres...)
			}
		} else {
			result = append(result, dPath)
		}
	}
	return result, err
}

// emptyLogFunc is a log function which do nothing for websocketServer.
func emptyLogFunc(l *libwebsocketd.LogScope, level libwebsocketd.LogLevel, levelName string, category string, msg string, args ...interface{}) {
	return
}

// buildGDBWebsocketHandler will build a websocket handler which execute gdb command.
func buildGDBWebsocketHandler(commandArgs []string) *libwebsocketd.WebsocketdServer {
	wsConfig := &libwebsocketd.Config{
		CommandName: "gdb",
		DevConsole:  true,
		CommandArgs: commandArgs,
	}
	webSocketHandler := libwebsocketd.NewWebsocketdServer(wsConfig, libwebsocketd.RootLogScope(0, emptyLogFunc), 6)
	return webSocketHandler
}
