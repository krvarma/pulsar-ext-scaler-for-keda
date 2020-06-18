# Developing external scaler for KEDA
![Apache Pulsar External Scaler](https://raw.githubusercontent.com/krvarma/pulsar-ext-scaler-for-keda/master/images/ext-scalar-keda.png?token=AA46XG65DJAK2MZTZMAVJ7263OV3C)
In my [previous article](https://medium.com/swlh/event-driven-autoscaling-of-kubernetes-workloads-using-keda-12524f8ade3), we looked into how to auto-scale an Azure Function with RabbitMQ trigger using KEDA. In this article, we will explore how to develop a custom external scaler for KEDA. In this article, we will create an external scaler for [Apache Pulsar](https://pulsar.apache.org/).

# What is KEDA?

KEDA is Kubernetes Event-Driven Autoscaling. KEDA is a lightweight component that determines when a container should be scaled. KEDA works along with Kubernetes’s Horizontal Pod Autoscaler. KEDA can scale any container in Kubernetes from 0 to n based on how many events are waiting to be processed.

KEDA can process Kubernetes Deployments and Jobs. Deployments are a common way to scale the workloads. When there is a long-running task, we have to use Kubernetes Jobs.

Some of the crucial concepts in KEDA are:
1.  **Scaling Agent**: The primary goal of the Scaling Agent is to activate and deactivate deployments.
2.  **Metrics Server**: The Metrics Server acts as a Kubernetes Metrics Server. Metrics Server sends scaling parameters like Queue Length, Number of messages in a stream to HPA so that HPA can initiate scaling.
3.  **Scaler**: Scalers are responsible for collecting metrics from an event source. The scale handler talks to the scaler to get the event metrics. Depends on the reported event metrics by the scaler, the scale handler scales the deployment or job. Scaler acts as a bridge between KEDA and external event sources. KEDA provides many built-in scalers. Kafka, RabbitMQ, AWS Kinesis, Azure Service Bus, NATS are some of the built-in scales.
4.  **ScaledObject**: ScaledObject is a Kubernetes Custom Resource Definition (CRD). We should deploy a ScaledaObject to scale a Deployment. ScaledObject defines the deployment name, minimum replicas, maximum replicas, metadata for scaler.

The scalers are an essential component in KEDA. It is the link between KEDA and the external event source. The scalers connect to the external event source and get the pending events to be processed. Scalers report this number to the scale handler component. KEDA provides many built-in scalers. We can write our custom scaler if there is not built-in scaler available.

# What is Apache Pulsar?

[Apache Pulsar](https://pulsar.apache.org/) is a pub-sub messaging system initially developed at Yahoo. It is now under Apache Software Foundation. Apache Pulsar supports many features that are required by cloud-native applications such as multi-tenancy, geo-replication, high scalability, etc. For more information about Apache Pulsar, visit their [home page](https://pulsar.apache.org/).

# Developing custom scaler for KEDA

There are two ways to write our custom scaler. One is to download the KEDA source code and develop our custom scaler, recompile, and install the binary. This method introduces many problems in upgrading and maintaining the KEDA installation. If there is a new version of the KEDA systems, then we need to download and compile the entire system. Added to that, if there are any breaking changes in the scaler architecture, then we should re-write our custom scaler.

KEDA provides a particular type of scaler called External Scaler, which makes the development of custom scalers easy. The external scaler connects to a gRPC server to connect to a custom event source. To develop and external scaler, we need to write a gRPC server and expose a service.

When the trigger type is external, the KEDA scaler connects to the gRPC endpoint and call some predefined methods. `New`, `GetMetricSpec`, `IsActive`, `GetMetrics`, and `Close` are the gRPC methods.

You can write an external scaler in any language that supports gRPC. The developer is responsible for managing the external scaler. In most of the cases, we will deploy the external scaler in the same cluster in which KEDA resides. The KEDA should be able to connect to the gRPC endpoint. So we should create a Kubernetes Deployment and a Service.

# External scaler for Apache Pulsar

For this article, we are going to create an external scaler for Apache Pulsar. We will be using Golang for development. We will use Pulsar Admin APIs to retrieve stats about a topic. Pulsar provides an Admin API to perform administration operations through which we can retrieve topic statistics. The Admin API URL to retrieve topic stats is like this:  

    http://<server><port>/admin/v2/<persistent/non-persistent>/<tenant>/<namespace>/<topic>/stats

An example is:

    http://localhost:8080/admin/v2/persistent/public/default/my-topic/stats

Assuming the server is running in localhost, we are using a persistent topic, the name of the tenant is public, the namespace is _default_, and the topic name is my-topic. Calling this URL will return a JSON document like this:

    {
       "msgRateIn":0.0,
       "msgThroughputIn":0.0,
       "msgRateOut":0.0,
       "msgThroughputOut":0.0,
       "averageMsgSize":0.0,
       "storageSize":0,
       "backlogSize":0,
       "publishers":[
    
       ],
       "subscriptions":{
          "first-subscription":{
             "msgRateOut":0.0,
             "msgThroughputOut":0.0,
             "msgRateRedeliver":0.0,
             "msgBacklog":0,
             "msgBacklogNoDelayed":0,
             "blockedSubscriptionOnUnackedMsgs":false,
             "msgDelayed":0,
             "unackedMessages":0,
             "type":"Exclusive",
             "activeConsumerName":"dec52",
             "msgRateExpired":0.0,
             "lastExpireTimestamp":0,
             "lastConsumedFlowTimestamp":1590687726940,
             "lastConsumedTimestamp":0,
             "lastAckedTimestamp":0,
             "consumers":[
                {
                   "msgRateOut":0.0,
                   "msgThroughputOut":0.0,
                   "msgRateRedeliver":0.0,
                   "consumerName":"dec52",
                   "availablePermits":1000,
                   "unackedMessages":0,
                   "blockedConsumerOnUnackedMsgs":false,
                   "lastAckedTimestamp":0,
                   "lastConsumedTimestamp":0,
                   "metadata":{
    
                   },
                   "address":"/192.168.86.36:53688",
                   "connectedSince":"2020-05-28T17:42:06.93Z",
                   "clientVersion":"2.5.1"
                }
             ],
             "isReplicated":false
          }
       },
       "replication":{
    
       },
       "deduplicationStatus":"Disabled",
       "bytesInCounter":0,
       "msgInCounter":0
    }

From the above JSON document, we can see that there is an array of subscriptions. These are the names of the subscriptions. Apache Pulsar supports four different subscription types. For more about subscriptions, read the [Apache Pulsar documentation](http://pulsar.apache.org/docs/en/concepts-messaging/#subscription-modes). Each subscription has a tag called msgBackLog, which represents the number of messages in the subscription backlog. If the number is greater than zero, it means there are some messages in the backlog. If it is zero, it means all messages are processed.

# External gRPC server

Let us look at the gRPC service in more detail. As mentioned in the previous section, the following are the methods we need to expose.

-   `New(ctx context.Context, newRequest *pb.NewRequest)  (*empty.Empty, error)` - Notify the external scaler that a new scaler is created and initialized. There are two parameters, `ctx` and `newRequest`. The parameters `ctx` is the package context; `newRequest` is an instance of the structure `NewRequest` defined in the protocol file. The `NewRequest` structure contains `ScaledObjectRef` and `Metadata`. The `ScaledObjectRef` structure contains information about the Scaled Object defined in the YAML file. The member Metadata is a map containing the metadata data defined on the YAML file. The Metadata member variable is of particular interest to us. We retrieve and save the user-defined metadata and also initialize resources, if any. The function returns empty pointer and error. If the `error` is not nil, then KEDA will not load our scaler.
-   `GetMetricsSpec(ctx context.Context, in *pb.ScaledObjectRef)  (*pb.GetMetricSpecResponse, error)` - Return a spec containing Metric Name and Target Size. KEDA use this spec to create the HPA. The function receives two parameters, a package context and a pointer to `ScaledObjectRef`. The function returns a pointer to the `GetMetricSpecResponse` instance, which contains a list or MetricSpec. You can return a value in error if there is an error occurred, otherwise return `nil`.
-   `IsActive(ctx context.Context, in *pb.ScaledObjectRef) (*pb.IsActiveResponse, error)` - KEDA system call this function to check whether it is OK to scale the deployment to zero. The function should return a pointer to the `IsActiveResponse` structure. The boolean member Result indicates whether it is OK to scale to zero or not. A value of true indicates there are pending items to process and should not scale to zero. A value of false indicates there are no pending items and can safely scale to zero. Normally we will get pending messages; if there are no pending messages, we will return true otherwise we will return false. You can return a value in error if there is an error occurred, otherwise return `nil`.
-   `GetMetrics(ctx context.Context, in *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error)` - KEDA call this function to get the metrics. The function should return a list of structure `MetricValue`. The structure `MetricValue` contains the name of the metric and its value. You should get pending events to be processed and return. You can return an error if there is an error occurred, otherwise return `nil`.
-   `Close(ctx context.Context, scaledObjectRef *pb.ScaledObjectRef) (*empty.Empty, error)` - Notify the Scaler the deployment is removed and closed. We can do any cleanup procedure here. You can return an error if there is an error occurred, otherwise return `nil`.

The .proto file looks like this:

    syntax = "proto3";
    
    option go_package = "github.com/krvarma/pulsar-ext-scaler/externalscaler";
    
    package externalscaler;
    
    import "google/protobuf/empty.proto";
    
    service ExternalScaler {
        rpc New(NewRequest) returns (google.protobuf.Empty) {}
        rpc IsActive(ScaledObjectRef) returns (IsActiveResponse) {}
        rpc GetMetricSpec(ScaledObjectRef) returns (GetMetricSpecResponse) {}
        rpc GetMetrics(GetMetricsRequest) returns (GetMetricsResponse) {}
        rpc Close(ScaledObjectRef) returns (google.protobuf.Empty) {}
    }
    
    message ScaledObjectRef {
        string name = 1;
        string namespace = 2;
    }
    
    message NewRequest {
        ScaledObjectRef scaledObjectRef = 1;
        map<string, string> metadata = 2;
    }
    
    message IsActiveResponse {
        bool result = 1;
    }
    
    message GetMetricSpecResponse {
        repeated MetricSpec metricSpecs = 1;
    }
    
    message MetricSpec {
        string metricName = 1;
        int64 targetSize = 2;
    }
    
    message GetMetricsRequest {
        ScaledObjectRef scaledObjectRef = 1;
        string metricName = 2;
    }
    
    message GetMetricsResponse {
        repeated MetricValue metricValues = 1;
    }
    
    message MetricValue {
        string metricName = 1;
        int64 metricValue = 2;
    }

To generate Golang files, we need to use the protocol buffer compiler, `protoc`. The following command will create the go file for our `.proto` file.

    protoc --go_out=plugins=grpc:. --go_opt=paths=source_relative externalscaler/externalscaler.proto

The above command will create the corresponding Golang file. Strictly speaking, we should not modify the `.go` file generated by the protoc compiler. But for the sake of simplicity, we will add some helper methods so that we can register the gRPC server easily.

Let us now look at the actual scaler code. Here is our actual external scaler code:

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
    	//url := getPulsarAdminStatsUrl("localhost:8080", true, "public", "default", "my-topic")
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

Other than the gRPC methods, we have some helper functions in this code. They are `getPulsarAdminStatsUrl`, `getMsgBackLogTag`, `getMsgBackLog`, and `parsePulsarMetadata`. These are just helper methods to call the Apache Pulsar Admin API and parse the user-supplied metadata.

In our `main` function, we start the gRPC server on the port specified in the environment variable.

# Build the external scaler and create a docker image
We will now create the docker image and push it to the registry so that we can deploy it on the Kubernetes cluster. For that, we will be using a multi-stage docker file. Our Dockerfile looks like this.

    FROM golang:alpine AS builder
    RUN mkdir /app
    ADD . /app/
    WORKDIR /app
    RUN go build -o pulsar-ext-scaler .
    
    FROM alpine
    RUN mkdir /app
    WORKDIR /app
    COPY --from=builder /app/pulsar-ext-scaler .
    
    CMD ["./pulsar-ext-scaler"]

Let us build and push the image to the registry.

    docker buildx build --platform linux/amd64 --push -t <docker username>/pulsar-ext-scaler .

Now we have the scaler image in the Docker registry and ready to deploy the scaler to the cluster.

Since we are exposing a server, we should create a deployment and a service. Our YAML file looks like this:

    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: keda-pulsar-ext-scaler
      namespace: default
    spec:
      selector:
        matchLabels:
          service: keda-pulsar-ext-scaler
      replicas: 1
      template:
        metadata:
          labels:
            service: keda-pulsar-ext-scaler
        spec:
          containers:
          - image: <replace with your external scaler image>
            name: scaler
            env:
            - name: EXTERNAL_SCALER_GRPC_PORT
              value: "8091"
    ---
    apiVersion: v1
    kind: Service
    metadata:
        name: pulsar-ext-scaler-service
        namespace: default
    spec:
        selector:
            service: keda-pulsar-ext-scaler
        type: ClusterIP
        ports:
        - protocol: TCP
          port: 8091

Apply the YAML file using the following command:

    kubectl apply -f pulsar-ext-scaler.yml

If everything goes well, you can see the external scaler pod running.

![External Scaler running](https://raw.githubusercontent.com/krvarma/pulsar-ext-scaler-for-keda/master/images/pulsar-ext-scaler-running.png?token=AA46XG364NQQGQRUIRRAUWK63OWBO)

Let us check the logs to see the server is running without any errors.

![External Scalar logs](https://raw.githubusercontent.com/krvarma/pulsar-ext-scaler-for-keda/master/images/pulsar-ext-scaler-logs.png?token=AA46XGZOBBRTSIABK3RVF5K63OWIY)

Now we have successfully deployed our external scaler to the cluster. The next step is to create an Apache Pulsar consumer so that we can test our scalar. We can use Golang using [Golang Pulsar Client SDK](https://github.com/apache/pulsar-client-go) to build a consumer. I modified one of the SDK example code to meet our needs. Here is the source code.

# Sample Apache Pulsar consumer
We can use Golang using [Golang Pulsar Client SDK](https://github.com/apache/pulsar-client-go) to build a consumer. I modified one of the SDK example code to meet our needs. Here is the source code.

    package main
    
    import (
    	"context"
    	"fmt"
    	"log"
    	"os"
    
    	"github.com/apache/pulsar-client-go/pulsar"
    )
    
    func getEnv(key string, defvalue string) string {
    	value := os.Getenv(key)
    
    	if len(value) <= 0 {
    		value = defvalue
    	}
    
    	return value
    }
    
    func main() {
    	server := getEnv("PULSAR_SERVER", "pulsar://localhost:6650")
    	topic := getEnv("PULSAR_TOPIC", "my-topic")
    	subscriptionName := getEnv("PULSAR_SUBSCRIPTION_NAME", "first-subscription")
    
    	log.Printf("Server: %v", server)
    	log.Printf("Topic: %v", topic)
    	log.Printf("Subscription: %v", subscriptionName)
    
    	client, err := pulsar.NewClient(pulsar.ClientOptions{URL: server})
    	if err != nil {
    		log.Fatal(err)
    	}
    
    	defer client.Close()
    
    	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
    		Topic:            topic,
    		SubscriptionName: subscriptionName,
    		Type:             pulsar.Shared,
    	})
    
    	if err != nil {
    		log.Fatal(err)
    	}
    
    	defer consumer.Close()
    
    	ctx := context.Background()
    
    	// Listen indefinitely on the topic
    	for {
    		msg, err := consumer.Receive(ctx)
    		if err != nil {
    			log.Fatal(err)
    		}
    
    		// Do something with the message
    		fmt.Printf("Message Received: %v\n", string(msg.Payload()))
    
    		if err == nil {
    			// Message processed successfully
    			consumer.Ack(msg)
    		} else {
    			// Failed to process messages
    			consumer.Nack(msg)
    		}
    	}
    }

As you can see, it a typical consumer. The only thing to note is that when subscribing, we are using the shared subscription type. As I mentioned in the previous section, Apache Pulsar supports four different subscription modes. *Exclusive*, *Shared*, *Failover*, and *Key Shared*. We are using the shared subscription mode. In this mode, multiple consumers can attach to a subscription and messages delivered in a round-robin distribution across consumers. In this mode, a message will send to only one consumer. Using this mode is very important since we need to distribute the workload to different instances.

Let us build this consumer and push it to the registry. Same as in the case of the external scaler, we will be using a multi-stage docker build. Our Dockerfile looks like this.

    FROM golang:alpine AS builder
    RUN mkdir /app
    ADD . /app/
    WORKDIR /app
    RUN go build -o consumer .
    
    FROM alpine
    RUN mkdir /app
    WORKDIR /app
    COPY --from=builder /app/consumer .
    
    ENV PULSAR_SERVER <replace with your pulsar server url>
    ENV PULSAR_TOPIC my-topic
    ENV PULSAR_SUBSCRIPTION_NAME first-subscription
    
    CMD ["./consumer"]

Now we have pushed our consumer to the docker registry. Let us deploy our consumer to the cluster and enable auto-scaling. Here is the YAML file for our `Deployment` and `ScaledObject`.

    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: pulsar-consumer
      namespace: default
    spec:
      selector:
        matchLabels:
          app: pulsar-consumer
      replicas: 1
      template:
        metadata:
          labels:
            app: pulsar-consumer
        spec:
          containers:
          - name: pulsar-consumer
            image: <insert your image name>
            imagePullPolicy: Always
    ---
    apiVersion: keda.k8s.io/v1alpha1
    kind: ScaledObject
    metadata:
        name: pulsar-consumer-scaledobject
        namespace: default
        labels:
            deploymentName: pulsar-consumer
    spec:
        pollingInterval: 10   # Optional. Default: 30 seconds
        cooldownPeriod: 100  # Optional. Default: 300 seconds
        minReplicaCount: 0   # Optional. Default: 0
        maxReplicaCount: 30  # Optional. Default: 100  
        scaleTargetRef:
            deploymentName: pulsar-consumer
        triggers:
        - type: external
          metadata:
            scalerAddress: pulsar-ext-scaler-service:8091
            server: "http://<server>:8080"
            persistent: "true"
            backlog: "10"
            tenant: "public"
            namespace: "default"
            topic: "my-topic"
            subscription: "first-subscription"    

The Deployment section is relatively simple. Let us look at the `ScaledObject` section. The name is `pulsar-consumer-scaledobject,` polling interval 10 seconds, cooldown period is 100 seconds, minimum replica count is 0, maximum replica count is 30, and the deployment name is `pulsar-consumer` (this is the name of our consumer deployment).

Next comes the trigger type. For external scalers, the trigger type is external. There is one more mandatory field, the scalerAddress, in the metadata section. The scalerAddress should be the Service URI of our scaler service. In our case, it is _pulsar-ext-scaler-service:8091_. All other metadata is our scaler specific. They are:

-   `server`:- This points to the Pulsar Admin server endpoint.
-   `persistent`:- True or False, specify whether the topic is persistent or non-persistent.
-   `backlog`:- Specify backlog value, scaling our deployment depends on this number. If you have 30 pending messages and the backlog is 10, then KEDA will create three pods to process these messages.
-   `tenant`:- Name of the tenant
-   `namespace`:- Name of the namespace
-   `topic`:- Topic name
-   `subscription`:- Name of the subscription

Let us deploy the pulsar consumer.

    kubectl apply -f pulsar-consumer-keda.yml

Once the deployment is over, you can check with `kubectl get pods` to check whether pods are created running. You can see that no pods are running; this is because KEDA scaled it to zero. You can check the deployments using the `kubectl get deploy` command.

![Scaler Deployment](https://raw.githubusercontent.com/krvarma/pulsar-ext-scaler-for-keda/master/images/pulsar-consumer-deployment.png?token=AA46XG3JCMF3KO5Y6YY32S263OWO2)

To send a message to the topic, we can use one of the example code of Apache Pulsar Go Client. Following is the modified code.

    package main
    
    import (
    	"context"
    	"fmt"
    	"log"
    	"os"
    	"time"
    
    	"github.com/apache/pulsar-client-go/pulsar"
    )
    
    func getEnv(key string, defvalue string) string {
    	value := os.Getenv(key)
    
    	if value == "" {
    		value = defvalue
    	}
    
    	return value
    }
    
    func main() {
    	server := getEnv("PULSAR_SERVER", "pulsar://localhost:6650")
    	message := getEnv("PULSAR_MESSAGE", "Sample Message")
    	topic := getEnv("PULSAR_TOPIC", "my-topic")
    
    	client, err := pulsar.NewClient(pulsar.ClientOptions{
    		URL:               server,
    		OperationTimeout:  30 * time.Second,
    		ConnectionTimeout: 30 * time.Second,
    	})
    
    	if err != nil {
    		log.Fatalf("Could not instantiate Pulsar client: %v", err)
    	}
    
    	defer client.Close()
    
    	producer, err := client.CreateProducer(pulsar.ProducerOptions{
    		Topic: topic,
    	})
    
    	_, err = producer.Send(context.Background(), &pulsar.ProducerMessage{
    		Payload: []byte(message),
    	})
    
    	defer producer.Close()
    
    	if err != nil {
    		fmt.Println("Failed to publish message", err)
    	}
    
    	fmt.Println("Published message")
    }

Let us send a message to a topic and see whether the external scaler picks it up and scale our consumer.

We can check by getting the pods. You can see the pods are created and running. You can wait for 100 seconds and verify it is terminating automatically.

![Scaler running and terminating](https://raw.githubusercontent.com/krvarma/pulsar-ext-scaler-for-keda/master/images/pulsar-consumer-runing-and-terminating.png?token=AA46XG7XRBR5EFTAXZ5B46C63OWSS)

We have successfully developed an external scaler. I hope this article gives you a gentle introduction to the external scaler for KEDA and how to develop one. You don't need to use Golang for the development, any language that supports gRPC can use for development.