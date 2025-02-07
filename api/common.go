package api

import (
	"allspark/cloud"
	"allspark/daemon"
	"allspark/logger"
	"allspark/monitor"
	"allspark/util/serializer"
	"errors"
	"io/ioutil"
	"net/http"
)

func validateRequest(r *http.Request, method string) error {
	if r.Method != method {
		return errors.New("invalid request method: " + r.Method)
	}

	if method == "POST" && r.Body == nil {
		return errors.New("form body is null")
	}

	return nil
}

func getStatus(w http.ResponseWriter, r *http.Request) {
	logger.GetDebug().Println("http-request: /status")
	err := validateRequest(r, "GET")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	clusterID := r.FormValue("clusterID")
	if len(clusterID) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("clusterID not specified"))
		return
	}
	logger.GetInfo().Printf("checking status on clusterID %v", clusterID)

	status := monitor.GetLastKnownStatus(clusterID)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(status))
}

func terminate(w http.ResponseWriter, r *http.Request, environment string) {
	err := validateRequest(r, "POST")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	clusterID := r.PostFormValue("clusterID")
	if len(clusterID) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("clusterID not specified"))
		return
	}

	logger.GetInfo().Println("handling termination request for clusterID: " + clusterID)

	clientBuffer, clientEnvironment, err := monitor.GetClientData(clusterID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Unable to retrieve status for clusterID " + clusterID))
		return
	}

	if clientEnvironment != environment {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("cloud environment does not match for clusterID " + clusterID))
		return
	}

	_, err = cloud.Create(clientEnvironment, clientBuffer)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Unable to establish allspark client with clusterID " + clusterID))
		return
	}

	monitor.SetCanceled(clusterID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("received cluster termination request"))
}

func checkIn(w http.ResponseWriter, r *http.Request) {
	logger.GetDebug().Println("http-request: /check-in")
	err := validateRequest(r, "POST")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	var body cloud.SparkStatusCheckIn
	buffer, err := ioutil.ReadAll(r.Body)
	serializer.Deserialize(buffer, &body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	logger.GetInfo().Printf("Form body: %s", buffer)

	monitor.HandleCheckIn(body.ClusterID, body.AppExitStatus, body.Status)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	logger.GetDebug().Println("http-request: /health-check")
	err := validateRequest(r, "GET")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}

// Init - initializes the allspark-orchestrator web api
func Init() {
	if daemon.GetAllSparkConfig().AwsEnabled {
		InitAwsAPI()
	}

	if daemon.GetAllSparkConfig().AzureEnabled {
		InitAzureAPI()
	}

	if daemon.GetAllSparkConfig().DockerEnabled {
		InitDockerAPI()
	}

	http.HandleFunc("/check-in", checkIn)
	http.HandleFunc("/status", getStatus)
	http.HandleFunc("/health-check", healthCheck)
	http.ListenAndServe(":32418", nil)
}
