package recoverer

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"syscall"

	"github.com/go-logr/logr"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// SignalRecoverer manages recovery that sending signal to processes.
type SignalRecoverer interface {
	Handler(http.ResponseWriter, *http.Request)
	ListSignals(diagnosisv1.Abnormal) (SignalList, error)
}

// signalRecovererImpl implements SignalRecoverer interface.
type signalRecovererImpl struct {
	// Context carries values across API boundaries.
	Context context.Context
	// Log represents the ability to log messages.
	Log logr.Logger
}

// SignalList contains details to send signals to processes.
type SignalList []Signal

// Signal contains details to send a signal to a process.
type Signal struct {
	PID    int            `json:"pid"`
	Signal syscall.Signal `json:"signal"`
}

// NewSignalRecoverer creates a new SignalRecoverer.
func NewSignalRecoverer(
	ctx context.Context,
	log logr.Logger,
) SignalRecoverer {
	return &signalRecovererImpl{
		Context: ctx,
		Log:     log,
	}
}

// Handler handles http requests for sending signal to processes.
func (pr *signalRecovererImpl) Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to read request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var abnormal diagnosisv1.Abnormal
		err = json.Unmarshal(body, &abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to unmarshal request body into an abnormal: %v", err), http.StatusNotAcceptable)
			return
		}

		// Get process signal details.
		signals, err := pr.ListSignals(abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get process signal details: %v", err), http.StatusInternalServerError)
			return
		}

		// Send signals to processes.
		for _, signal := range signals {
			pr.Log.Info("sending signal to process", "process", signal.PID, "signal", signal.Signal)
			err := syscall.Kill(signal.PID, signal.Signal)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to send signal %d to process %d: %v", signal.Signal, signal.PID, err), http.StatusInternalServerError)
				return
			}
		}

		data, err := json.Marshal(abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal abnormal: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// ListSignals list process signal details by retrieving context in abnormal.
func (pr *signalRecovererImpl) ListSignals(abnormal diagnosisv1.Abnormal) (SignalList, error) {
	pr.Log.Info("listing signals")

	data, err := util.GetAbnormalSpecContext(abnormal, util.SignalRecoveryContextKey)
	if err != nil {
		return nil, err
	}

	var signals SignalList
	err = json.Unmarshal(data, &signals)
	if err != nil {
		return nil, err
	}

	return signals, nil
}
