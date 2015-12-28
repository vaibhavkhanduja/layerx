package layerx_rpi_api
import (
	"github.com/layer-x/layerx-commons/lxactionqueue"
	"github.com/layer-x/layerx-core_v2/layerx_rpi_client"
	"github.com/go-martini/martini"
"net/http"
"io/ioutil"
	"github.com/layer-x/layerx-commons/lxerrors"
"github.com/mesos/mesos-go/scheduler"
	"github.com/Sirupsen/logrus"
"github.com/layer-x/layerx-commons/lxlog"
	"github.com/layer-x/layerx-mesos-rpi_v2/layerx_rpi_api/rpi_api_helpers"
	"encoding/json"
)

const (
	COLLECT_RESOURCES = "/collect_resources"
	LAUNCH_TASKS = "/launch_tasks"
	KILL_TASK = "/kill_task"
)

var empty = []byte{}

type rpiApiServerWrapper struct {
	actionQueue      lxactionqueue.ActionQueue
	rpi              *layerx_rpi_client.LayerXRpi
	mesosSchedulerDriver scheduler.SchedulerDriver
}


func NewRpiApiServerWrapper(rpi *layerx_rpi_client.LayerXRpi, mesosSchedulerDriver scheduler.SchedulerDriver, actionQueue lxactionqueue.ActionQueue) *rpiApiServerWrapper {
	return &rpiApiServerWrapper{
		actionQueue: actionQueue,
		mesosSchedulerDriver: mesosSchedulerDriver,
		rpi: rpi,
	}
}

func (wrapper *rpiApiServerWrapper) WrapWithRpi(m *martini.ClassicMartini, masterUpidString string, driverErrc chan error) *martini.ClassicMartini {
	collectResourcesHandler := func(req *http.Request, res http.ResponseWriter) {
		collectResourcesFn := func() ([]byte, int, error) {
			err := rpi_api_helpers.CollectResources(wrapper.mesosSchedulerDriver)
			if err != nil {
				lxlog.Errorf(logrus.Fields{
					"error": err,
				}, "could not handle collect resources request")
				return empty, 500, lxerrors.New("could not handle collect resources request", err)
			}
			return empty, 202, nil
		}
		_, statusCode, err := wrapper.queueOperation(collectResourcesFn)
		if err != nil {
			res.WriteHeader(statusCode)
			lxlog.Errorf(logrus.Fields{
				"error": err.Error(),
				"request_sent_by": masterUpidString,
			}, "processing collect resources message")
			driverErrc <- err
			return
		}
		res.WriteHeader(statusCode)
	}
	launchTasksHandler := func(req *http.Request, res http.ResponseWriter) {
		launchTasksFn := func() ([]byte, int, error) {
			data, err := ioutil.ReadAll(req.Body)
			if req.Body != nil {
				defer req.Body.Close()
			}
			if err != nil {
				return empty, 400, lxerrors.New("parsing launch tasks request", err)
			}
			var launchTasksMessage layerx_rpi_client.LaunchTasksMessage
			err = json.Unmarshal(data, &launchTasksMessage)
			if err != nil {
				return empty, 500, lxerrors.New("could not parse json to update task status message", err)
			}
			err = rpi_api_helpers.LaunchTasks(wrapper.mesosSchedulerDriver, launchTasksMessage)
			if err != nil {
				lxlog.Errorf(logrus.Fields{
					"error": err,
				}, "could not handle launch tasks request")
				return empty, 500, lxerrors.New("could not handle launch tasks request", err)
			}
			return empty, 202, nil
		}
		_, statusCode, err := wrapper.queueOperation(launchTasksFn)
		if err != nil {
			res.WriteHeader(statusCode)
			lxlog.Errorf(logrus.Fields{
				"error": err.Error(),
				"request_sent_by": masterUpidString,
			}, "processing launch tasks message")
			driverErrc <- err
			return
		}
		res.WriteHeader(statusCode)
	}
	killTaskHandler := func(req *http.Request, res http.ResponseWriter, params martini.Params) {
		killTaskFn := func() ([]byte, int, error) {
			taskId := params["task_id"]
			err := rpi_api_helpers.KillTask(wrapper.mesosSchedulerDriver, taskId)
			if err != nil {
				lxlog.Errorf(logrus.Fields{
					"error": err,
				}, "could not handle kill task request")
				return empty, 500, lxerrors.New("could not handle kill task request", err)
			}
			return empty, 202, nil
		}
		_, statusCode, err := wrapper.queueOperation(killTaskFn)
		if err != nil {
			res.WriteHeader(statusCode)
			lxlog.Errorf(logrus.Fields{
				"error": err.Error(),
				"request_sent_by": masterUpidString,
			}, "processing kill task message")
			driverErrc <- err
			return
		}
		res.WriteHeader(statusCode)
	}

	m.Post(COLLECT_RESOURCES, collectResourcesHandler)
	m.Post(LAUNCH_TASKS, launchTasksHandler)
	m.Post(KILL_TASK+"/:task_id", killTaskHandler)
	return m
}

func (wrapper *rpiApiServerWrapper) queueOperation(f func() ([]byte, int, error)) ([]byte, int, error) {
	datac := make(chan []byte)
	statusCodec := make(chan int)
	errc := make(chan error)
	wrapper.actionQueue.Push(
		func() {
			data, statusCode, err := f()
			datac <- data
			statusCodec <- statusCode
			errc <- err
		})
	return <-datac, <-statusCodec, <-errc
}
