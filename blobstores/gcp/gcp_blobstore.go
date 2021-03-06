package gcp

import (
	"context"
	"io"
	"strings"
	"time"

	"golang.org/x/oauth2/jwt"

	"cloud.google.com/go/storage"

	"github.com/cenkalti/backoff"
	"github.com/cloudfoundry-incubator/bits-service"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/validate"
	"github.com/cloudfoundry-incubator/bits-service/config"
	"github.com/cloudfoundry-incubator/bits-service/logger"
	"github.com/cloudfoundry-incubator/bits-service/util"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type Blobstore struct {
	client       *storage.Client
	jwtConfig    *jwt.Config
	bucket       string
	retryTimeout time.Duration
}

func NewBlobstore(config config.GCPBlobstoreConfig) *Blobstore {
	validate.NotEmpty(config.Bucket)
	validate.NotEmpty(config.Email)
	validate.NotEmpty(config.PrivateKey)
	validate.NotEmpty(config.PrivateKeyID)
	validate.NotEmpty(config.TokenURL)

	ctx := context.TODO()

	jwtConfig := &jwt.Config{
		Email:        config.Email,
		PrivateKey:   []byte(config.PrivateKey),
		PrivateKeyID: config.PrivateKeyID,
		Scopes:       []string{storage.ScopeFullControl},
		TokenURL:     config.TokenURL,
	}
	client, err := storage.NewClient(ctx, option.WithTokenSource(jwtConfig.TokenSource(ctx)))
	if err != nil {
		panic(err)
	}
	if config.RetryTimeoutSeconds == 0 {
		config.RetryTimeoutSeconds = 5
	}
	return &Blobstore{
		client:       client,
		bucket:       config.Bucket,
		jwtConfig:    jwtConfig,
		retryTimeout: time.Duration(config.RetryTimeoutSeconds) * time.Second,
	}
}

// The Golang GCP API is limited in the way it does retries:
// By default, it retries forever, with no safe-guard against "hanging" requests. By decorating
// the context passed to all functions with a timeout, it's now safe-guarded, but there is now
// no way to cut off connections and retry. It's because the timeout decorator now causes the retry
// mechanism to break out of the retry loop. The retry mechanism was not written with "hanging"
// requests in mind, that simply need to be cut off and retried.
// Therefore, we must add the retry here in all functions ontop of the built-in retry.

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), blobstore.retryTimeout)
	defer cancel()

	e := WithRetries(4, func() error {
		_, e := blobstore.client.Bucket(blobstore.bucket).Object(path).Attrs(ctx)
		return TimeoutOrPermanent(e)
	})
	if e != nil {
		e = blobstore.handleError(e, "Failed to check for %v/%v", blobstore.bucket, path)
		if _, ok := e.(*bitsgo.NotFoundError); ok {
			return false, nil
		}
		return false, e
	}
	return true, nil
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	logger.Log.Debugw("Get from GCP", "bucket", blobstore.bucket, "path", path)
	reader, e := blobstore.client.Bucket(blobstore.bucket).Object(path).NewReader(context.TODO())
	if e != nil {
		return nil, blobstore.handleError(e, "Path %v", path)
	}
	return reader, nil
}

func (blobstore *Blobstore) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	signedUrl, e := storage.SignedURL(blobstore.bucket, path, &storage.SignedURLOptions{
		GoogleAccessID: blobstore.jwtConfig.Email,
		PrivateKey:     blobstore.jwtConfig.PrivateKey,
		Method:         "GET",
		Expires:        time.Now().Add(time.Hour),
	})
	return nil, signedUrl, e
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	logger.Log.Debugw("Put to GCP", "bucket", blobstore.bucket, "path", path)
	if e := blobstore.bucketExists(); e != nil {
		return e
	}
	writer := blobstore.client.Bucket(blobstore.bucket).Object(path).NewWriter(context.TODO())
	var safeCloser util.SafeCloser
	defer safeCloser.Close(writer)

	_, e := io.Copy(writer, src)
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}

	e = safeCloser.Close(writer)
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	logger.Log.Debugw("Copy in GCP", "bucket", blobstore.bucket, "src", src, "dest", dest)

	ctx, cancel := context.WithTimeout(context.TODO(), blobstore.retryTimeout)
	defer cancel()

	e := WithRetries(4, func() error {
		_, e := blobstore.client.Bucket(blobstore.bucket).Object(dest).CopierFrom(blobstore.client.Bucket(blobstore.bucket).Object(src)).Run(ctx)
		return TimeoutOrPermanent(e)
	})
	if e != nil {
		return blobstore.handleError(e, "Error while trying to copy src %v to dest %v in bucket %v", src, dest, blobstore.bucket)
	}
	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), blobstore.retryTimeout)
	defer cancel()

	e := WithRetries(4, func() error {
		e := blobstore.client.Bucket(blobstore.bucket).Object(path).Delete(ctx)
		return TimeoutOrPermanent(e)
	})
	if e != nil {
		return blobstore.handleError(e, "Path %v", path)
	}
	return nil
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	deletionErrs := []error{}
	it := blobstore.client.Bucket(blobstore.bucket).Objects(context.TODO(), &storage.Query{Prefix: prefix})
	for {
		attrs, e := it.Next()
		if e == iterator.Done {
			break
		}
		if e != nil {
			return errors.Wrapf(e, "Prefix %v", prefix)
		}
		e = blobstore.Delete(attrs.Name)
		if e != nil {
			if _, isNotFoundError := e.(*bitsgo.NotFoundError); !isNotFoundError {
				deletionErrs = append(deletionErrs, e)
			}
		}
	}
	if len(deletionErrs) != 0 {
		return errors.Errorf("Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	return nil
}

func (blobstore *Blobstore) Sign(resource string, method string, expirationTime time.Time) (signedURL string) {
	if strings.ToLower(method) != "get" && method != "put" {
		panic("The only supported methods are 'put' and 'get'")
	}
	signedURL, e := storage.SignedURL(blobstore.bucket, resource, &storage.SignedURLOptions{
		GoogleAccessID: blobstore.jwtConfig.Email,
		PrivateKey:     blobstore.jwtConfig.PrivateKey,
		Method:         strings.ToUpper(method),
		Expires:        expirationTime,
	})
	if e != nil {
		panic(e)
	}
	logger.Log.Debugw("Signed URL", "verb", method, "signed-url", signedURL)
	return
}

func WithRetries(numRetries uint64, f func() error) error {
	return backoff.Retry(f, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), numRetries))
}

func TimeoutOrPermanent(e error) error {
	if timeoutError, isTimeoutError := e.(interface{ Timeout() bool }); isTimeoutError && timeoutError.Timeout() {
		return e
	}
	return backoff.Permanent(e)
}

func (blobstore *Blobstore) handleError(e error, context string, args ...interface{}) error {
	if e == storage.ErrObjectNotExist {
		e := blobstore.bucketExists()
		if e != nil {
			return e
		}
		return bitsgo.NewNotFoundError()
	}
	return errors.Wrapf(e, context, args...)
}

func (blobstore *Blobstore) bucketExists() error {
	_, e := blobstore.client.Bucket(blobstore.bucket).Attrs(context.TODO())
	if e != nil {
		return errors.Wrapf(e, "Error while checking for bucket existence. Bucket '%v'", blobstore.bucket)
	}
	return nil
}
