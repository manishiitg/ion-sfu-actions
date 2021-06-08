package tracktodisk

import (
	"context"
	"os"

	"cloud.google.com/go/storage"
	avp "github.com/pion/ion-avp/pkg"
	"github.com/pion/ion-avp/pkg/elements"
	log "github.com/pion/ion-log"
)

// FileWriter instance
type FileWriter struct {
	elements.Leaf
	wr     *storage.Writer
	client *storage.Client
}

// NewFileWriter instance
// bufSize is the buffer size in bytes. Pass <=0 to disable buffering.
func NewCloudFileWriter(path string) *FileWriter {

	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "./cfgs/gcloud/steady-datum-291915-9c9286662fbf.json")
	projectID := "steady-datum-291915"
	client, err := storage.NewClient(context.Background())
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
	}
	bucketName := "golivexsfutodisk"
	bucket := client.Bucket(bucketName)

	storageClassAndLocation := &storage.BucketAttrs{
		StorageClass: "STANDARD",
		Location:     "asia",
	}
	ctx := context.Background()
	fw := &FileWriter{}
	if err := bucket.Create(ctx, projectID, storageClassAndLocation); err != nil {
		log.Errorf("Bucket(%v).Create: %v", bucketName, err)
	}

	w := bucket.Object(path).NewWriter(ctx)
	fw.wr = w
	fw.client = client
	return fw
}

func (w *FileWriter) Write(sample *avp.Sample) error {
	_, err := w.wr.Write(sample.Payload.([]byte))
	if err != nil {
		log.Errorf("file write error %v", err)
	}
	return err
}

func (w *FileWriter) Close() {
	err := w.wr.Close()
	w.client.Close()
	if err != nil {
		log.Errorf("close error %v", err)
	}
}
