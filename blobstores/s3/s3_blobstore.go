package s3

import (
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/logger"
	"github.com/petergtz/bitsgo/routes"
	"github.com/pkg/errors"
	"github.com/uber-go/zap"
)

type LegacyBlobStore struct {
	pureRedirect *PureRedirectBlobStore
	noRedirect   *NoRedirectBlobStore
}

func NewLegacyBlobstore(config config.S3BlobstoreConfig) *LegacyBlobStore {
	return &LegacyBlobStore{
		pureRedirect: NewPureRedirectBlobstore(config),
		noRedirect:   NewNoRedirectBlobStore(config),
	}
}

func (blobstore *LegacyBlobStore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	return blobstore.pureRedirect.Get(path)
}

func (blobstore *LegacyBlobStore) Head(path string) (redirectLocation string, err error) {
	return blobstore.pureRedirect.Head(path)
}

func (blobstore *LegacyBlobStore) Put(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	return blobstore.noRedirect.Put(path, src)
}

func (blobstore *LegacyBlobStore) Copy(src, dest string) (redirectLocation string, err error) {
	return blobstore.noRedirect.Copy(src, dest)
}

func (blobstore *LegacyBlobStore) Exists(path string) (bool, error) {
	return blobstore.noRedirect.Exists(path)
}

func (blobstore *LegacyBlobStore) Delete(path string) error {
	return blobstore.noRedirect.Delete(path)
}

func (blobstore *LegacyBlobStore) DeletePrefix(prefix string) error {
	return blobstore.noRedirect.DeletePrefix(prefix)
}

type PureRedirectBlobStore struct {
	s3Client *s3.S3
	bucket   string
}

func NewPureRedirectBlobstore(config config.S3BlobstoreConfig) *PureRedirectBlobStore {
	return &PureRedirectBlobStore{
		s3Client: newS3Client(config.Region, config.AccessKeyID, config.SecretAccessKey),
		bucket:   config.Bucket,
	}
}

func (blobstore *PureRedirectBlobStore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedUrl, e := signedURLFrom(request, blobstore.bucket, path)
	return nil, signedUrl, e
}

func (blobstore *PureRedirectBlobStore) Head(path string) (redirectLocation string, err error) {
	// TODO: this is actually wrong, but the client requires this contract right now it seems.
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	return signedURLFrom(request, blobstore.bucket, path)
}

func (blobstore *PureRedirectBlobStore) Put(path string, src io.Reader) (redirectLocation string, err error) {
	request, _ := blobstore.s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	return signedURLFrom(request, blobstore.bucket, path)
}

func (blobstore *PureRedirectBlobStore) Copy(src, dest string) (redirectLocation string, err error) {
	request, _ := blobstore.s3Client.CopyObjectRequest(&s3.CopyObjectInput{
		Key:        &dest,
		CopySource: &src,
		Bucket:     &blobstore.bucket,
	})
	return signedURLFrom(request, blobstore.bucket, dest)
}

func signedURLFrom(req *request.Request, bucket, path string) (string, error) {
	signedURL, e := req.Presign(time.Hour)
	if e != nil {
		return "", errors.Wrapf(e, "Bucket/Path %v/%v", bucket, path)
	}
	return signedURL, nil

}

func (blobstore *PureRedirectBlobStore) Delete(path string) error {
	_, e := blobstore.s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}

type NoRedirectBlobStore struct {
	s3Client *s3.S3
	bucket   string
}

func NewNoRedirectBlobStore(config config.S3BlobstoreConfig) *NoRedirectBlobStore {
	return &NoRedirectBlobStore{
		s3Client: newS3Client(config.Region, config.AccessKeyID, config.SecretAccessKey),
		bucket:   config.Bucket,
	}
}

func (blobstore *NoRedirectBlobStore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	logger.Log.Debug("Get from S3", zap.String("bucket", blobstore.bucket), zap.String("path", path))
	output, e := blobstore.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return nil, "", routes.NewNotFoundError()
		}
		return nil, "", errors.Wrapf(e, "Path %v", path)
	}
	return output.Body, "", nil
}

func (blobstore *NoRedirectBlobStore) Head(path string) (redirectLocation string, err error) {
	logger.Log.Debug("Head from S3", zap.String("bucket", blobstore.bucket), zap.String("path", path))
	_, e := blobstore.s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return "", routes.NewNotFoundError()
		}
		return "", errors.Wrapf(e, "Path %v", path)
	}
	return "", nil
}

func (blobstore *NoRedirectBlobStore) Put(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	logger.Log.Debug("Put to S3", zap.String("bucket", blobstore.bucket), zap.String("path", path))
	_, e := blobstore.s3Client.PutObject(&s3.PutObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
		Body:   src,
	})
	if e != nil {
		return "", errors.Wrapf(e, "Path %v", path)
	}
	return "", nil
}

func (blobstore *NoRedirectBlobStore) Copy(src, dest string) (redirectLocation string, err error) {
	logger.Log.Debug("Copy in S3", zap.String("bucket", blobstore.bucket), zap.String("src", src), zap.String("dest", dest))
	_, e := blobstore.s3Client.CopyObject(&s3.CopyObjectInput{
		Key:        &dest,
		CopySource: aws.String(blobstore.bucket + "/" + src),
		Bucket:     &blobstore.bucket,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return "", routes.NewNotFoundError()
		}
		return "", errors.Wrapf(e, "Error while trying to copy src %v to dest %v in bucket %v", src, dest, blobstore.bucket)
	}
	return "", nil
}

func (blobstore *NoRedirectBlobStore) Exists(path string) (bool, error) {
	_, e := blobstore.s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return false, nil
		}
		return false, errors.Wrapf(e, "Failed to check for %v/%v", blobstore.bucket, path)
	}
	return true, nil
}

func (blobstore *NoRedirectBlobStore) Delete(path string) error {
	_, e := blobstore.s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return routes.NewNotFoundError()
		}
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}

func (blobstore *NoRedirectBlobStore) DeletePrefix(prefix string) error {
	deletionErrs := []error{}
	e := blobstore.s3Client.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket: &blobstore.bucket,
			Prefix: &prefix,
		},
		func(p *s3.ListObjectsOutput, lastPage bool) (shouldContinue bool) {
			for _, object := range p.Contents {
				e := blobstore.Delete(*object.Key)
				if e != nil {
					if _, isNotFoundError := e.(*routes.NotFoundError); !isNotFoundError {
						deletionErrs = append(deletionErrs, e)
					}
				}
			}
			return true
		})
	if e != nil {
		return errors.Wrapf(e, "Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	if len(deletionErrs) != 0 {
		return errors.Errorf("Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	return nil
}

func isS3NotFoundError(e error) bool {
	if ae, isAwsErr := e.(awserr.Error); isAwsErr {
		if ae.Code() == "NoSuchBucket" || ae.Code() == "NoSuchKey" || ae.Code() == "NotFound" {
			return true
		}
	}
	return false
}