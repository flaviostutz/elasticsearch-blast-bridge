package elasticblast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

//METRICS
var invocationHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "blast_invocation",
	Help:    "Blast invocations",
	Buckets: []float64{0.1, 1, 10},
}, []string{
	"method",
	"url",
	"status",
})

var blastURL = ""

func initBlast(blastURL0 string) {
	prometheus.MustRegister(invocationHist)
	blastURL = blastURL0
}

func storeDocument(contents map[string]interface{}) error {
	logrus.Debugf("createDocument %v", contents)
	wfb, err := json.Marshal(contents)
	if err != nil {
		return err
	}

	url := "/v1/documents"
	resp, data, err := postHTTP(url, wfb, "/v1/documents")
	if err != nil {
		logrus.Errorf("Call to Blast POST %s failed. err=%s", url, err)
		return err
	}
	if resp.StatusCode != 200 {
		logrus.Warnf("POST %s call status!=200. resp=%v", url, resp)
		return fmt.Errorf("Failed to create new document. status=%d", resp.StatusCode)
	}
	logrus.Debugf("Document created successfully. data=%v", data)

	return nil
}

func loadDocument(id string) (map[string]interface{}, int, error) {
	logrus.Debugf("loadDocument %s", id)

	resp, data, err := getHTTP(fmt.Sprintf("%s/v1/documents/%s", blastURL, id), "/v1/documents")
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("GET %s/v1/documents/%s. err=%s", blastURL, id, err)
	}
	if resp.StatusCode >= 500 {
		return nil, resp.StatusCode, fmt.Errorf("Error getting document. id=%s. status=%d", id, resp.StatusCode)
	}

	var docdata map[string]interface{}
	err = json.Unmarshal(data, &docdata)
	if err != nil {
		logrus.Errorf("Error parsing json. err=%s", err)
		return nil, resp.StatusCode, err
	}
	return docdata, resp.StatusCode, nil
}

func searchDocument(query map[string]interface{}) (map[string]interface{}, error) {
	logrus.Debugf("searchDocument %v", query)

	b, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	resp, data, err := postHTTP(fmt.Sprintf("%s/v1/search", blastURL), b, "/v1/search")
	if err != nil {
		return nil, fmt.Errorf("POST %s/v1/search. err=%s", blastURL, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Couldn't search documents. status=%d", resp.StatusCode)
	}

	var docdata map[string]interface{}
	err = json.Unmarshal(data, &docdata)
	if err != nil {
		logrus.Errorf("Error parsing json. err=%s", err)
		return nil, err
	}
	return docdata, nil
}

func postHTTP(url string, data []byte, metricsInfo string) (http.Response, []byte, error) {
	startTime := time.Now()
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		logrus.Errorf("HTTP request creation failed. err=%s", err)
		return http.Response{}, []byte{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: time.Second * 10,
	}
	logrus.Debugf("POST request=%v", req)
	response, err1 := client.Do(req)
	if err1 != nil {
		logrus.Errorf("HTTP request invocation failed. err=%s", err1)
		return http.Response{}, []byte{}, err1
	}

	logrus.Debugf("Response: %v", response)
	datar, _ := ioutil.ReadAll(response.Body)
	logrus.Debugf("Response body: %s", datar)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		logrus.Debugf("%s status code not ok. status_code=%d", metricsInfo, response.StatusCode)
	}
	invocationHist.WithLabelValues("POST", metricsInfo, fmt.Sprintf("%d", response.StatusCode)).Observe(float64(time.Since(startTime).Seconds()))

	return *response, datar, nil
}

func getHTTP(url0 string, metricsInfo string) (http.Response, []byte, error) {
	startTime := time.Now()
	req, err := http.NewRequest("GET", url0, nil)
	if err != nil {
		logrus.Errorf("HTTP request creation failed. err=%s", err)
		return http.Response{}, []byte{}, err
	}

	client := &http.Client{
		Timeout: time.Second * 10,
	}
	logrus.Debugf("GET request=%v", req)
	response, err1 := client.Do(req)
	if err1 != nil {
		logrus.Errorf("HTTP request invocation failed. err=%s", err1)
		invocationHist.WithLabelValues(metricsInfo, "error").Observe(float64(time.Since(startTime).Seconds()))
		return http.Response{}, []byte{}, err1
	}

	// logrus.Debugf("Response: %v", response)
	datar, _ := ioutil.ReadAll(response.Body)
	logrus.Debugf("Response body: %s", datar)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		logrus.Debugf("%s status code not ok. status_code=%d", metricsInfo, response.StatusCode)
	}
	invocationHist.WithLabelValues("GET", metricsInfo, fmt.Sprintf("%d", response.StatusCode)).Observe(float64(time.Since(startTime).Seconds()))

	return *response, datar, nil
}
