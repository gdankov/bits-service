package s3_blobstore_test

import (
	"testing"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	. "github.com/petergtz/bitsgo/s3_blobstore"
)

func TestS3Blobstore(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "S3Blobstore")
}

var _ = Describe("Signing URLs", func() {

	It("Can create pre-signed URLs for S3", func() {
		signer := NewS3BuildpackCacheSigner("mybucket", "MY-Key_ID", "dummy", "us-east-1")

		signedURL := signer.Sign("/my/path", "get")

		Expect(signedURL).To(SatisfyAll(
			ContainSubstring("https://mybucket.s3.amazonaws.com/my/path"),
			ContainSubstring("X-Amz-Algorithm="),
			ContainSubstring("X-Amz-Credential=MY-Key_ID"),
			ContainSubstring("X-Amz-Date="),
			ContainSubstring("X-Amz-Expires="),
			ContainSubstring("X-Amz-Signature="),
		))
	})

	It("Can create pre-signed URLs for S3", func() {
		signer := NewS3ResourceSigner("mybucket", "MY-Key_ID", "dummy", "us-east-1")

		signedURL := signer.Sign("myresource", "get")

		Expect(signedURL).To(SatisfyAll(
			ContainSubstring("https://mybucket.s3.amazonaws.com/my/re/myresource"),
			ContainSubstring("X-Amz-Algorithm="),
			ContainSubstring("X-Amz-Credential=MY-Key_ID"),
			ContainSubstring("X-Amz-Date="),
			ContainSubstring("X-Amz-Expires="),
			ContainSubstring("X-Amz-Signature="),
		))
	})
})
