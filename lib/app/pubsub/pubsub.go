// +build linux darwin
package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"flag"

	"cloud.google.com/go/pubsub"
	"github.com/golang/glog"
	"github.com/jkerry/sensu_gcp_pubsub_handler/lib/pkg/eventprocessing"
)

func SendMetric() error {
	client, topic, err := configure()
	if err != nil {
		glog.Errorf("Could not configure pubsub Client: %v", err)
		return err
	}
	event, err := eventprocessing.GetPipedEvent()
	if err != nil {
		glog.Errorf("Could not process or validate event data from stdin: %v", err)
		return err
	}

	for _, point := range event.Metrics.Points {
		metric, err := eventprocessing.GetMetricFromPoint(point, event.Entity.Name, event.Entity.Namespace, event.Entity.Labels)
		if err != nil {
			glog.Errorf("error processing sensu event MetricPoints into MetricValue: %v", err)
			return err
		}
		msg, err := json.Marshal(metric)
		if err != nil {
			glog.Errorf("error serializing metric data to pub/sub json payload: %v", err)
			return err
		}
		publish(client, topic, msg)
	}
	return nil
}

func configure() (*pubsub.Client, string, error) {
	var proj = flag.String("project_id", "", "the project id for the GCP PubSub topic.")
	var topic = flag.String("topic", "", "the project id for the GCP PubSub topic.")
	var validatePermissions = flag.Bool("test_permissions", false, "set to test pubsub publish permissions.")
	flag.Parse()
	ctx := context.Background()
	if *proj == "" {
		glog.Fatalf("project_id option must be set.")
		return nil, "", errors.New("project_id is required")
	}

	if *topic == "" {
		glog.Fatalf("topic option must be set.")
		return nil, "", errors.New("topic is required")
	}

	client, err := pubsub.NewClient(ctx, *proj)
	if err != nil {
		glog.Errorf("Could not create pubsub Client: %v", err)
		return nil, "", err
	}
	if *validatePermissions {
		_, err := testPermissions(client, *topic)
		if err != nil {
			glog.Errorf("Failed to fetch pubsub permissions or missing update/publish rights to the topic. %v", err)
			return nil, "", err
		}
	}
	return client, *topic, nil
}

func testPermissions(c *pubsub.Client, topicName string) ([]string, error) {
	ctx := context.Background()
	topic := c.Topic(topicName)
	perms, err := topic.IAM().TestPermissions(ctx, []string{
		"pubsub.topics.publish",
		"pubsub.topics.update",
	})
	if err != nil {
		return nil, err
	}
	return perms, nil
}

func publish(client *pubsub.Client, topic string, payload []byte) error {
	ctx := context.Background()
	t := client.Topic(topic)
	result := t.Publish(ctx, &pubsub.Message{
		Data: payload,
	})
	id, err := result.Get(ctx)
	if err != nil {
		return err
	}
	glog.Infof("Published a message; msg ID: %v\n", id)
	return nil
}
