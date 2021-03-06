package framework_manager

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/emc-advanced-dev/layerx/layerx-mesos-tpi/mesos_master_api/mesos_data"
	"github.com/emc-advanced-dev/pkg/errors"
	"github.com/gogo/protobuf/proto"
	"github.com/layer-x/layerx-commons/lxhttpclient"
	"github.com/mesos/mesos-go/mesosproto"
	"github.com/pborman/uuid"
)

type FrameworkManager interface {
	NotifyFrameworkRegistered(frameworkName, frameworkId string, frameworkUpid *mesos_data.UPID) error
	SendStatusUpdate(frameworkId string, frameworkUpid *mesos_data.UPID, status *mesosproto.TaskStatus) error
	SendTaskCollectionOffer(frameworkId, phonyOfferId, phonySlaveId, phonySlavePid string, frameworkUpid *mesos_data.UPID) error
	HealthCheckFramework(frameworkId string, frameworkUpid *mesos_data.UPID) (bool, error)
}

type frameworkManager struct {
	masterUpid *mesos_data.UPID
}

func NewFrameworkManager(masterUpid *mesos_data.UPID) *frameworkManager {
	return &frameworkManager{
		masterUpid: masterUpid,
	}
}

//notify a framework that it has successfully registered with the tpi
func (manager *frameworkManager) NotifyFrameworkRegistered(frameworkName, frameworkId string, frameworkUpid *mesos_data.UPID) error {
	if frameworkName == "" {
		return errors.New("framework must be named", nil)
	}

	masterState := &mesos_data.MesosState{
		Version: mesos_data.MESOS_VERSION,
		Leader:  manager.masterUpid.String(),
	}
	masterInfo, err := masterState.ToMasterInfo()
	if err != nil {
		return errors.New("converting master state to master info", err)
	}

	frameworkRegisteredMsg := &mesosproto.FrameworkRegisteredMessage{
		FrameworkId: &mesosproto.FrameworkID{
			Value: proto.String(frameworkId),
		},
		MasterInfo: masterInfo,
	}
	resp, _, err := manager.sendMessage(frameworkUpid, frameworkRegisteredMsg, "/mesos.internal.FrameworkRegisteredMessage")
	if err != nil {
		return errors.New("sending registered message to framework", err)
	}
	if !(resp.StatusCode == 200 || resp.StatusCode == 202) {
		statusCode := fmt.Sprintf("%v", resp.StatusCode)
		return errors.New("expected 200 or 202 response from framework, got "+statusCode, nil)
	}
	return nil
}

//send status update to framework
func (manager *frameworkManager) SendStatusUpdate(frameworkId string, frameworkUpid *mesos_data.UPID, status *mesosproto.TaskStatus) error {
	var executorId *mesosproto.ExecutorID
	if status.GetExecutorId() != nil {
		executorId = status.GetExecutorId()
	}
	var slaveId *mesosproto.SlaveID
	if status.GetSlaveId() != nil {
		slaveId = status.GetSlaveId()
	}
	statusUpdateUuid := uuid.New()
	statusUpdateMessage := &mesosproto.StatusUpdateMessage{
		Update: &mesosproto.StatusUpdate{
			FrameworkId: &mesosproto.FrameworkID{
				Value: proto.String(frameworkId),
			},
			ExecutorId:  executorId,
			SlaveId:     slaveId,
			Status:      status,
			Timestamp:   proto.Float64(float64(time.Now().Unix())),
			LatestState: status.State,
			Uuid:        []byte(statusUpdateUuid),
		},
	}
	resp, _, err := manager.sendMessage(frameworkUpid, statusUpdateMessage, "/mesos.internal.StatusUpdateMessage")
	if err != nil {
		return errors.New("sending status update to framework", err)
	}
	if !(resp.StatusCode == 200 || resp.StatusCode == 202) {
		statusCode := fmt.Sprintf("%v", resp.StatusCode)
		return errors.New("expected 200 or 202 response from framework, got "+statusCode, nil)
	}
	return nil
}

func (manager *frameworkManager) SendTaskCollectionOffer(frameworkId, phonyOfferId, phonySlaveId, phonySlavePid string, frameworkUpid *mesos_data.UPID) error {
	logrus.WithFields(logrus.Fields{
		"frameworkid": frameworkId,
	}).Debugf("sending task collection (phony) offer to framework")

	taskCollectionOffer := newPhonyOffer(frameworkId, phonyOfferId, phonySlaveId)
	offerMessage := &mesosproto.ResourceOffersMessage{
		Offers: []*mesosproto.Offer{taskCollectionOffer},
		Pids:   []string{phonySlavePid},
	}
	resp, _, err := manager.sendMessage(frameworkUpid, offerMessage, "/mesos.internal.ResourceOffersMessage")
	if err != nil {
		return errors.New("sending task collection offer to framework", err)
	}
	if !(resp.StatusCode == 200 || resp.StatusCode == 202) {
		statusCode := fmt.Sprintf("%v", resp.StatusCode)
		return errors.New("expected 200 or 202 response from framework, got "+statusCode, nil)
	}
	return nil
}

func (manager *frameworkManager) HealthCheckFramework(frameworkId string, frameworkUpid *mesos_data.UPID) (bool, error) {
	logrus.WithFields(logrus.Fields{
		"frameworkid": frameworkId,
	}).Debugf("checking health of framework")
	url := frameworkUpid.Host + ":" + frameworkUpid.Port
	_, _, err := lxhttpclient.Get(url, "/", nil)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return false, nil
		}
		return false, errors.New("performing health check on framework", err)
	}
	return true, nil
}

func (manager *frameworkManager) sendMessage(destination *mesos_data.UPID, message proto.Message, path string) (*http.Response, []byte, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := destination.Host + ":" + destination.Port
	path = "/" + destination.ID + path
	headers := map[string]string{
		"Libprocess-From": manager.masterUpid.String(),
		"Content-Type":    "application/json",
	}
	resp, data, err := lxhttpclient.Post(url, path, headers, message)
	if err != nil {
		err = errors.New("sending data("+string(data)+") to framework", err)
	}
	return resp, data, err
}
