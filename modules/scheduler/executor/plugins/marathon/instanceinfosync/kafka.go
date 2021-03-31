package instanceinfosync

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/sirupsen/logrus"

	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/modules/scheduler/conf"
	"github.com/erda-project/erda/modules/scheduler/instanceinfo"
)

const (
	measurementAllRunningContainers = "metaserver_all_containers"
	measurementContainer            = "metaserver_container"
)

type Syncer struct {
	dbupdater *instanceinfo.Client
}

func NewSyncer(db *instanceinfo.Client) *Syncer {
	return &Syncer{
		dbupdater: db,
	}
}

func (s *Syncer) Sync() {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":        conf.KafkaBrokers(),
		"group.id":                 conf.KafkaGroup(),
		"auto.offset.reset":        "latest",
		"auto.commit.interval.ms":  1000,
		"message.send.max.retries": 10000000,
	})
	if err != nil {
		logrus.Errorf("failed to create new kafka consumer: %v", err)
		return
	}
	defer func() {
		if err = c.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	contianerTopic := conf.KafkaContainerTopic()
	topics := []string{contianerTopic}
	logrus.Infof("cmdb topics: %v", topics)
	if err := c.SubscribeTopics(topics, nil); err != nil {
		logrus.Errorf("failed to subscribe kafka topics: %v", err)
		return
	}
	s.gc()
	for {
		msg, err := c.ReadMessage(-1)
		if err != nil {
			logrus.Errorf("failed to read kafka message: %v", err)
			continue
		}
		switch *msg.TopicPartition.Topic {
		case contianerTopic:
			s.processContainer(msg)
		default:
			logrus.Warnf("invalid kafka topic: %s", *msg.TopicPartition.Topic)
		}
	}
}
func (s *Syncer) gc() {
	go func() {
		for {
			time.Sleep(20 * time.Minute)
			if err := gcAliveInstancesInDB(s.dbupdater, 3600*4); err != nil {
				logrus.Errorf("failed to gcAliveInstancesInDB: %v", err)
			}
		}
	}()
}

// MetaserverMSG kafka容器信息格式
type MetaserverMSG struct {
	Name      string                 `json:"name"`      // metaserver_container、metaserver_all_containers
	TimeStamp int64                  `json:"timestamp"` // 纳秒
	Fields    map[string]interface{} `json:"fields"`    // 全量容器事件时 key: containerID value: container info
	Tags      map[string]string      `json:"tags,omitempty"`
}

var MsgTimeout = 3 * time.Minute

func isEdas(fields map[string]interface{}) bool {
	edasAppID, ok := fields["edas_app_id"]
	if !ok {
		return false
	}
	return edasAppID.(string) != ""
}

func isDCOS(fields map[string]interface{}) bool {
	return fields["task_id"].(string) != fields["id"].(string)
}

func isK8S(fields map[string]interface{}) bool {
	_, ok := fields["edas_app_id"]
	return fields["task_id"].(string) == fields["id"].(string) && !ok
}

func (s *Syncer) processContainer(msg *kafka.Message) {
	var containerMsg MetaserverMSG
	if err := json.Unmarshal(msg.Value, &containerMsg); err != nil {
		logrus.Errorf("failed to unmarshal container message: %v", err)
		return
	}
	switch containerMsg.Name {
	case measurementContainer:
		instance := convertToInstance(containerMsg.Fields)
		if isK8S(containerMsg.Fields) {
			return
		}
		if err := createOrUpdate(s.dbupdater, instance); err != nil {
			logrus.Errorf("failed to createOrUpdate instance: %v", err)
			return
		}
	case measurementAllRunningContainers:
		msgStart := time.Unix(containerMsg.TimeStamp/int64(time.Second/time.Nanosecond), 0)
		delayTime := time.Now().Sub(msgStart)
		if delayTime > MsgTimeout {
			logrus.Warningf("all running containers message expired, delaySeconds: %vs, timeout: %vs, msg: %+v",
				delayTime.Seconds(), MsgTimeout.Seconds(), containerMsg)
			return
		}
		for _, v := range containerMsg.Fields {
			var fields map[string]interface{}
			if err := json.Unmarshal([]byte(v.(string)), &fields); err != nil {
				logrus.Errorf("failed to unmarshal all container fields: %v", err)
				return
			}
			instance := convertToInstance(fields)
			if isK8S(fields) {
				continue
			}
			if err := createOrUpdate(s.dbupdater, instance); err != nil {
				logrus.Errorf("failed to createOrUpdate instance: %v", err)
				return
			}
		}
	}
}

func createOrUpdate(db *instanceinfo.Client, instance instanceinfo.InstanceInfo) error {
	currentInstancesInK8s, err := db.InstanceReader().ByTaskID(apistructs.K8S).ByContainerID(instance.ContainerID).Do()
	if err != nil {
		return err
	}
	if len(currentInstancesInK8s) != 0 {
		return nil
	}

	currentInstances, err := db.InstanceReader().ByTaskID(instance.TaskID).Do()
	if err != nil {
		return err
	}
	if len(currentInstances) == 0 {
		if err := db.InstanceWriter().Create(&instance); err != nil {
			return err
		}
	}
	for _, curr := range currentInstances {
		instance.ID = curr.ID
		instance.Phase = InstancestatusStateMachine(curr.Phase, instance.Phase)
		if err := db.InstanceWriter().Update(instance); err != nil {
			return err
		}
	}

	// clear duplicate instance in db
	// dcos有 2 处代码会去新建 instance(in db),
	// 1. marathon event
	// 2. 当前代码
	// -------------
	// 然而这里可能会有同步问题, 会出现2条container_id一样的记录,
	// (注意,container_id不能设置为unique,因为marathon event处没有container_id)
	// 所以需要处理一下重复记录
	idmap := map[string]uint64{}
	idToDel := []uint64{}
	currentInstances, err = db.InstanceReader().ByTaskID(instance.TaskID).Do()
	for _, ins := range currentInstances {
		if _, ok := idmap[ins.ContainerID]; ok {
			idToDel = append(idToDel, ins.ID)
		}
		idmap[ins.ContainerID] = ins.ID
	}
	if err := db.InstanceWriter().Delete(idToDel...); err != nil {
		return err
	}
	return nil
}

func convertToInstance(fields map[string]interface{}) instanceinfo.InstanceInfo {
	var instance instanceinfo.InstanceInfo
	if id, ok := fields["id"]; ok {
		instance.ContainerID = id.(string)
	}
	if ip, ok := fields["ip"]; ok {
		instance.ContainerIP = ip.(string)
	}
	if cluster, ok := fields["cluster_name"]; ok {
		instance.Cluster = cluster.(string)
	}
	if host, ok := fields["host_ip"]; ok {
		instance.HostIP = host.(string)
	}
	if startedAt, ok := fields["started_at"]; ok {
		utc, err := time.Parse(time.RFC3339Nano, startedAt.(string))
		if err == nil {
			instance.StartedAt = utc.Local()
		}
	}
	if finishedAt, ok := fields["finished_at"]; ok && !strings.Contains(finishedAt.(string), "0001-01-01T00:00:00Z") {
		utc, err := time.Parse(time.RFC3339Nano, finishedAt.(string))
		if err == nil {
			finishedTime := utc.Local()
			instance.FinishedAt = &finishedTime
		}
	}
	if image, ok := fields["image"]; ok {
		instance.Image = image.(string)
	}
	if cpu, ok := fields["cpu"]; ok {
		instance.CpuLimit = round(cpu.(float64), 2)
	}
	if memory, ok := fields["memory"]; ok {
		instance.MemLimit = (int)(memory.(float64)) / 1024 / 1024
	}
	// if disk, ok := fields["disk"]; ok {
	// 	container.Disk = (int64)(disk.(float64))
	// }
	if exitCode, ok := fields["exit_code"]; ok {
		instance.ExitCode = (int)(exitCode.(float64))
	}
	// if privileged, ok := fields["privileged"]; ok {
	// 	container.Privileged = privileged.(bool)
	// }
	if status, ok := fields["status"]; ok {
		instance.Phase = convertStatus(status.(string))
		if _, ok := fields["edas_app_id"]; ok {
			// 如果是 edas 的实例, 则直接将 running 作为 healthy
			if instance.Phase == instanceinfo.InstancePhaseRunning {
				instance.Phase = instanceinfo.InstancePhaseHealthy
			}
		}
	}
	if diceOrg, ok := fields["dice_org"]; ok {
		instance.OrgID = diceOrg.(string)
	}
	if diceProject, ok := fields["dice_project"]; ok {
		instance.ProjectID = diceProject.(string)
	}
	if diceApplication, ok := fields["dice_application"]; ok {
		instance.ApplicationID = diceApplication.(string)
	}
	if diceRuntime, ok := fields["dice_runtime"]; ok {
		instance.RuntimeID = diceRuntime.(string)
	}
	if diceService, ok := fields["dice_service"]; ok {
		instance.ServiceName = diceService.(string)
	}
	if edasAppID, ok := fields["edas_app_id"]; ok {
		if instance.Meta == "" {
			instance.Meta = fmt.Sprintf("edas_app_id=%s", edasAppID.(string))
		} else {
			instance.Meta = fmt.Sprintf("%s,edas_app_id=%s", instance.Meta, edasAppID.(string))
		}
	}
	if edasAppName, ok := fields["edas_app_name"]; ok {
		if instance.Meta == "" {
			instance.Meta = fmt.Sprintf("edas_app_name=%s", edasAppName.(string))
		} else {
			instance.Meta = fmt.Sprintf("%s,edas_app_name=%s", instance.Meta, edasAppName.(string))
		}
	}
	if edasGroupID, ok := fields["edas_group_id"]; ok {
		if instance.Meta == "" {
			instance.Meta = fmt.Sprintf("edas_group_id=%s", edasGroupID.(string))
		} else {
			instance.Meta = fmt.Sprintf("%s,edas_group_id=%s", instance.Meta, edasGroupID.(string))
		}
	}
	if diceProjectName, ok := fields["dice_project_name"]; ok {
		instance.ProjectName = diceProjectName.(string)
	}
	if diceApplicationName, ok := fields["dice_application_name"]; ok {
		instance.ApplicationName = diceApplicationName.(string)
	}
	if diceRuntimeName, ok := fields["dice_runtime_name"]; ok {
		instance.RuntimeName = diceRuntimeName.(string)
	}
	if diceComponent, ok := fields["dice_component"]; ok {
		if instance.Meta == "" {
			instance.Meta = fmt.Sprintf("dice_component=%s", diceComponent.(string))
		} else {
			instance.Meta = fmt.Sprintf("%s,dice_component=%s", instance.Meta, diceComponent.(string))
		}
	}
	if addonID, ok := fields["dice_addon"]; ok {
		instance.ServiceType = "addon"
		instance.AddonID = addonID.(string)
	}
	if _, ok := fields["dice_addon_name"]; ok {
		instance.ServiceType = "addon"
	}
	if _, ok := fields["pipeline_id"]; ok {
		instance.ServiceType = "job"
	}
	if diceWorkspace, ok := fields["dice_workspace"]; ok {
		instance.Workspace = diceWorkspace.(string)
	}
	if diceSharedLevel, ok := fields["dice_shared_level"]; ok {
		if instance.Meta == "" {
			instance.Meta = fmt.Sprintf("dice_shared_level=%s", diceSharedLevel.(string))
		} else {
			instance.Meta = fmt.Sprintf("%s,dice_shared_level=%s", instance.Meta, diceSharedLevel.(string))
		}
	}
	if taskID, ok := fields["task_id"]; ok {
		instance.TaskID = taskID.(string)
	}
	if instance.ServiceType == "" {
		instance.ServiceType = "stateless-service"
	}
	return instance
}

func round(f float64, n int) float64 {
	shift := math.Pow(10, float64(n))
	fv := 0.0000000001 + f //对浮点数产生.xxx999999999 计算不准进行处理
	return math.Floor(fv*shift+.5) / shift
}

func convertStatus(status string) instanceinfo.InstancePhase {
	switch status {
	case "Starting":
		return instanceinfo.InstancePhaseRunning
	case "Killed":
		return instanceinfo.InstancePhaseDead
	case "Stopped":
		return instanceinfo.InstancePhaseDead
	case "Healthy":
		return instanceinfo.InstancePhaseHealthy
	case "OOM":
		return instanceinfo.InstancePhaseDead
	default:
		return instanceinfo.InstancePhaseUnHealthy
	}
}

//                                         current                       expect                          result
var instancestatusStateMachineMap = map[instanceinfo.InstancePhase]map[instanceinfo.InstancePhase]instanceinfo.InstancePhase{
	instanceinfo.InstancePhaseRunning: {
		instanceinfo.InstancePhaseRunning:   instanceinfo.InstancePhaseRunning,
		instanceinfo.InstancePhaseHealthy:   instanceinfo.InstancePhaseHealthy,
		instanceinfo.InstancePhaseUnHealthy: instanceinfo.InstancePhaseUnHealthy,
		instanceinfo.InstancePhaseDead:      instanceinfo.InstancePhaseDead,
	},
	instanceinfo.InstancePhaseHealthy: {
		instanceinfo.InstancePhaseRunning:   instanceinfo.InstancePhaseHealthy,
		instanceinfo.InstancePhaseHealthy:   instanceinfo.InstancePhaseHealthy,
		instanceinfo.InstancePhaseUnHealthy: instanceinfo.InstancePhaseUnHealthy,
		instanceinfo.InstancePhaseDead:      instanceinfo.InstancePhaseDead,
	},
	instanceinfo.InstancePhaseUnHealthy: {
		instanceinfo.InstancePhaseRunning:   instanceinfo.InstancePhaseUnHealthy,
		instanceinfo.InstancePhaseHealthy:   instanceinfo.InstancePhaseHealthy,
		instanceinfo.InstancePhaseUnHealthy: instanceinfo.InstancePhaseUnHealthy,
		instanceinfo.InstancePhaseDead:      instanceinfo.InstancePhaseDead,
	},
	instanceinfo.InstancePhaseDead: {
		instanceinfo.InstancePhaseRunning:   instanceinfo.InstancePhaseDead,
		instanceinfo.InstancePhaseHealthy:   instanceinfo.InstancePhaseDead,
		instanceinfo.InstancePhaseUnHealthy: instanceinfo.InstancePhaseDead,
		instanceinfo.InstancePhaseDead:      instanceinfo.InstancePhaseDead,
	},
}

func InstancestatusStateMachine(currentStatus,
	expectNextStatus instanceinfo.InstancePhase) (resultStatus instanceinfo.InstancePhase) {
	return instancestatusStateMachineMap[currentStatus][expectNextStatus]
}

// func getInstanceByContainerID(cluster, containerID string) ([]instanceinfo.InstanceInfo, error) {
// 	var instances []instanceinfo.InstanceInfo
// }