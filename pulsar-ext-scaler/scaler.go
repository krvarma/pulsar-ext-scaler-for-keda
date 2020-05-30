package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"

	"github.com/golang/protobuf/ptypes/empty"
	pb "github.com/krvarma/pulsar-ext-scaler/externalscaler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type externalScalerServer struct {
	scaledObjectRef map[string][]*pb.ScaledObjectRef
}

// Pulsar Context
type PulsarContext struct {
	pulsarserver string // Pulsar URL
	isPersistent bool   // persistent or non-persistent
	tenant       string // tenant
	namespace    string // namespace
	topic        string // topic name
	subscription string // subscription name
	metricname   string // Metric Name
	targetsize   int    // Target size
	serverurl    string // Server URl
}

// Constant variables
const (
	pulsarurlpattern        = "%v/admin/v2/%v/%v/%v/%v/stats" // Pulsar URL format string
	pulsarmsgbacklogpattern = "subscriptions.%v.msgBacklog"   // JSON Tag format string
)

// Pulsar context
var (
	pc PulsarContext
)

// Get environment variable, if it is not found return default value
func getEnv(key string, defvalue string) string {
	value := os.Getenv(key)

	if len(value) <= 0 {
		value = defvalue
	}

	return value
}

// getPulsarAdminStatsUrl construct the Pulsar admin URL for getting the topic status and returns.
// The URL is constructed from the paramters.
func getPulsarAdminStatsUrl(server string, isPersistent bool, tenant string, namespace string, topic string) string {
	pstring := "persistent"

	if !isPersistent {
		pstring = "non-persistent"
	}

	if !strings.HasPrefix(server, "https") && !strings.HasPrefix(server, "http") {
		server = "http://" + server
	}

	return fmt.Sprintf(pulsarurlpattern, server, pstring, tenant, namespace, topic)
}

// getMsgBackLogTag JSON path that represents the topic backlog number
func getMsgBackLogTag(subscription string) string {
	return fmt.Sprintf(pulsarmsgbacklogpattern, subscription)
}

// getMsgBackLog calls the Stats admin API and returns the backlog number reported
// by the server
func getMsgBackLog(pulsarurl string, subscription string) int {
	log.Printf("Server URL %v", pulsarurl)
	response, err := http.Get(pulsarurl)

	var backlog = 0

	if err == nil {
		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
		} else {
			tag := getMsgBackLogTag(subscription)
			value := gjson.Get(string(responseData), tag)

			log.Printf("Backlog %v", value)

			if value.String() != "" {
				log.Print(value)

				ival, err := strconv.Atoi(value.String())

				if err != nil {
					log.Error(err)
				} else {
					backlog = ival
				}
			}
		}
	} else {
		log.Error(err)
	}

	return backlog
}

// Parse Pulsar Metadata
func parsePulsarMetadata(metadata map[string]string) {
	pc.pulsarserver = metadata["server"]

	ispersistent, err := strconv.ParseBool(metadata["persistent"])

	if err != nil {
		pc.isPersistent = ispersistent
	} else {
		pc.isPersistent = true
	}

	ts, err := strconv.Atoi(metadata["backlog"])

	if err != nil {
		pc.targetsize = ts
	} else {
		pc.targetsize = 10
	}

	pc.tenant = metadata["tenant"]
	pc.namespace = metadata["namespace"]
	pc.topic = metadata["topic"]
	pc.subscription = metadata["subscription"]
	pc.metricname = pc.tenant + "-" + pc.namespace + "-" + pc.topic
	pc.serverurl = getPulsarAdminStatsUrl(pc.pulsarserver, pc.isPersistent, pc.tenant, pc.namespace, pc.topic)

}

// NewRequest
func (s *externalScalerServer) New(ctx context.Context, newRequest *pb.NewRequest) (*empty.Empty, error) {
	out := new(empty.Empty)

	parsePulsarMetadata(newRequest.Metadata)

	return out, nil
}

// Close
func (s *externalScalerServer) Close(ctx context.Context, scaledObjectRef *pb.ScaledObjectRef) (*empty.Empty, error) {
	out := new(empty.Empty)

	return out, nil
}

// IsActive
func (s *externalScalerServer) IsActive(ctx context.Context, in *pb.ScaledObjectRef) (*pb.IsActiveResponse, error) {
	backlog := getMsgBackLog(pc.serverurl, pc.subscription)

	return &pb.IsActiveResponse{
		Result: backlog > 0,
	}, nil
}

func (s *externalScalerServer) GetMetricSpec(ctx context.Context, in *pb.ScaledObjectRef) (*pb.GetMetricSpecResponse, error) {
	log.Info("Getting Metric Spec...")
	spec := pb.MetricSpec{
		MetricName: pc.metricname,
		TargetSize: int64(pc.targetsize),
	}

	log.Printf("GetMetricSpec() method completed for %s", spec.MetricName)

	return &pb.GetMetricSpecResponse{
		MetricSpecs: []*pb.MetricSpec{&spec},
	}, nil
}

func (s *externalScalerServer) GetMetrics(ctx context.Context, in *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	backlog := getMsgBackLog(pc.serverurl, pc.subscription)

	m := pb.MetricValue{
		MetricName:  pc.metricname,
		MetricValue: int64(backlog),
	}

	log.Printf("GetMetrics() method completed for %s", m.MetricName)

	return &pb.GetMetricsResponse{
		MetricValues: []*pb.MetricValue{&m},
	}, nil
}

func newServer() *externalScalerServer {
	s := &externalScalerServer{}

	return s
}

func main() {
	//url := getPulsarAdminStatsUrl("ubuntuserver:8080", true, "public", "default", "my-topic")
	//value := getMsgBackLog(url, "first-subscription")

	//log.Printf("Backlog %v\n", value)

	port := getEnv("EXTERNAL_SCALER_GRPC_PORT", "8091")

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%v", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	} else {
		log.Printf("gRPC server running on %v", port)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	reflection.Register(grpcServer)
	pb.RegisterExternalScalerServer(grpcServer, newServer())
	grpcServer.Serve(lis)
}
