package common

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	pb "github.com/beam-cloud/beta9/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type attachmentClientStream struct {
	pb.ContainerService_ContainerStreamLogsClient
	attach <-chan struct{}
	err    error
}

func (s *attachmentClientStream) Header() (metadata.MD, error) {
	<-s.attach
	return nil, s.err
}

func (s *attachmentClientStream) Recv() (*pb.ContainerLogEntry, error) {
	return nil, io.EOF
}

type attachmentContainerClient struct {
	pb.ContainerServiceClient
	stream pb.ContainerService_ContainerStreamLogsClient
}

func (c *attachmentContainerClient) ContainerStreamLogs(context.Context, *pb.ContainerStreamLogsRequest, ...grpc.CallOption) (pb.ContainerService_ContainerStreamLogsClient, error) {
	return c.stream, nil
}

func TestStreamLogsReadyWaitsForWorkerAttachment(t *testing.T) {
	attach := make(chan struct{})
	ready := make(chan struct{})
	client := &ContainerClient{client: &attachmentContainerClient{
		stream: &attachmentClientStream{attach: attach},
	}}
	done := make(chan error, 1)
	go func() {
		done <- client.StreamLogsWithReady(context.Background(), "container-id", make(chan OutputMsg), func() { close(ready) })
	}()

	select {
	case <-ready:
		t.Fatal("reported ready before worker attachment")
	case <-time.After(20 * time.Millisecond):
	}

	close(attach)
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("did not report worker attachment")
	}
	require.NoError(t, <-done)
}

func TestStreamLogsReadyReportsAttachmentFailure(t *testing.T) {
	attach := make(chan struct{})
	close(attach)
	ready := make(chan struct{})
	attachErr := errors.New("attachment failed")
	client := &ContainerClient{client: &attachmentContainerClient{
		stream: &attachmentClientStream{attach: attach, err: attachErr},
	}}

	err := client.StreamLogsWithReady(context.Background(), "container-id", make(chan OutputMsg), func() { close(ready) })

	select {
	case <-ready:
	default:
		t.Fatal("did not report failed worker attachment")
	}
	require.ErrorIs(t, err, attachErr)
}
