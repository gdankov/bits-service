package main_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	s3sdk "github.com/aws/aws-sdk-go/service/s3"
	bitsgo "github.com/cloudfoundry-incubator/bits-service"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/alibaba"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/azure"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/gcp"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/openstack"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/s3"
	"github.com/cloudfoundry-incubator/bits-service/config"
	"github.com/cloudfoundry-incubator/bits-service/httputil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Non-local blobstores", func() {

	var (
		filepath     string
		srcFilepath  string
		destFilepath string
		blobstore    blobstore
	)

	itCanPutAndGetAResourceThere := func() {

		It("can put and get a resource there", func() {
			Expect(blobstore.Exists(filepath)).To(BeFalse())

			body, e := blobstore.Get(filepath)
			Expect(e).To(BeAssignableToTypeOf(&bitsgo.NotFoundError{}))
			Expect(body).To(BeNil())

			body, redirectLocation, e := blobstore.GetOrRedirect(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(body).To(BeNil())
			Expect(http.Get(redirectLocation)).To(HaveStatusCode(http.StatusNotFound))

			e = blobstore.Put(filepath, strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())

			Expect(blobstore.Exists(filepath)).To(BeTrue())

			body, e = blobstore.Get(filepath)
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(body)).To(ContainSubstring("the file content"))

			body, redirectLocation, e = blobstore.GetOrRedirect(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(body).To(BeNil())
			Expect(http.Get(redirectLocation)).To(HaveBodyWithSubstring("the file content"))

			e = blobstore.Delete(filepath)
			Expect(e).NotTo(HaveOccurred())

			Expect(blobstore.Exists(filepath)).To(BeFalse())

			body, e = blobstore.Get(filepath)
			Expect(e).To(BeAssignableToTypeOf(&bitsgo.NotFoundError{}))
			Expect(body).To(BeNil())

			body, redirectLocation, e = blobstore.GetOrRedirect(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(body).To(BeNil())
			Expect(http.Get(redirectLocation)).To(HaveStatusCode(http.StatusNotFound))
		})

		Describe("DeleteDir", func() {
			BeforeEach(func() {
				e := blobstore.Put("one", strings.NewReader("the file content"))
				Expect(e).NotTo(HaveOccurred())

				e = blobstore.Put("two", strings.NewReader("the file content"))
				Expect(e).NotTo(HaveOccurred())

				Expect(blobstore.Exists("one")).To(BeTrue())
				Expect(blobstore.Exists("two")).To(BeTrue())
			})

			AfterEach(func() {
				blobstore.Delete("one")
				blobstore.Delete("two")
				Expect(blobstore.Exists("one")).To(BeFalse())
				Expect(blobstore.Exists("two")).To(BeFalse())
			})

			It("Can delete a prefix", func() {
				e := blobstore.DeleteDir("")
				Expect(e).NotTo(HaveOccurred())

				Expect(blobstore.Exists("one")).To(BeFalse())
				Expect(blobstore.Exists("two")).To(BeFalse())
			})
		})

		Context("Copy", func() {
			BeforeEach(func() {
				srcFilepath = fmt.Sprintf("src-testfile")
				destFilepath = fmt.Sprintf("dest-testfile")
				body, e := blobstore.Get(srcFilepath)
				Expect(e).To(BeAssignableToTypeOf(&bitsgo.NotFoundError{}))
				Expect(body).To(BeNil())
				e = blobstore.Put(srcFilepath, strings.NewReader("the file content"))
				Expect(e).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				e := blobstore.Delete(srcFilepath)
				Expect(e).NotTo(HaveOccurred())
				e = blobstore.Delete(destFilepath)
				Expect(e).NotTo(HaveOccurred())
			})

			It("copies a resource from src to dest", func() {
				e := blobstore.Copy(srcFilepath, destFilepath)
				Expect(e).NotTo(HaveOccurred())

				body, e := blobstore.Get(destFilepath)
				Expect(e).NotTo(HaveOccurred())
				Expect(body).NotTo(BeNil())
			})
		})

		It("Can delete a prefix like in a file tree", func() {
			Expect(blobstore.Exists("dir/one")).To(BeFalse())
			Expect(blobstore.Exists("dir/two")).To(BeFalse())

			e := blobstore.Put("dir/one", strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())
			e = blobstore.Put("dir/two", strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())

			Expect(blobstore.Exists("dir/one")).To(BeTrue())
			Expect(blobstore.Exists("dir/two")).To(BeTrue())

			e = blobstore.DeleteDir("dir")
			Expect(e).NotTo(HaveOccurred())

			Expect(blobstore.Exists("dir/one")).To(BeFalse())
			Expect(blobstore.Exists("dir/two")).To(BeFalse())
		})

		It("can get a signed PUT URL and upload something to it", func() {
			signedUrl := blobstore.Sign(filepath, "put", time.Now().Add(1*time.Hour))

			r := httputil.NewRequest("PUT", signedUrl, strings.NewReader("the file content"))

			// The following line is a hack to make Azure work.
			// (See:
			// https://stackoverflow.com/questions/37824136/put-on-sas-blob-url-without-specifying-x-ms-blob-type-header
			// https://stackoverflow.com/questions/16160045/azure-rest-webclient-put-blob
			// https://stackoverflow.com/questions/12711150/unable-to-upload-file-image-n-vdo-to-blob-storage-getting-error-mandatory-he)

			// Not a huge problem, since we decided that all uploads must go through the bits-service anyway. But still annoying.
			r.WithHeader("x-ms-blob-type", "BlockBlob")

			response, e := http.DefaultClient.Do(r.Build())
			Expect(e).NotTo(HaveOccurred())

			Expect(response.StatusCode).To(Or(Equal(http.StatusOK), Equal(http.StatusCreated)))
		})
	}

	ItDoesNotReturnNotFoundError := func() {
		It("does not throw a NotFoundError", func() {
			_, e := blobstore.Get("irrelevant-path")
			Expect(e).NotTo(BeAssignableToTypeOf(&bitsgo.NotFoundError{}))
		})
	}

	var configFileContent []byte

	BeforeEach(func() {
		filename := os.Getenv("CONFIG")
		if filename == "" {
			fmt.Println("No $CONFIG set. Defaulting to integration_test_config.yml")
			filename = "integration_test_config.yml"
		}
		file, e := os.Open(filename)
		Expect(e).NotTo(HaveOccurred())
		defer file.Close()
		configFileContent, e = ioutil.ReadAll(file)
		Expect(e).NotTo(HaveOccurred())

		filepath = fmt.Sprintf("testfile-%v", time.Now())
	})

	Context("S3", func() {
		var s3Config config.S3BlobstoreConfig

		BeforeEach(func() {
			s3Config = config.S3BlobstoreConfig{}
			Expect(yaml.Unmarshal(configFileContent, &s3Config)).To(Succeed())
		})

		JustBeforeEach(func() { blobstore = s3.NewBlobstore(s3Config) })

		Context("Without Server Side Encryption", func() {
			BeforeEach(func() {
				// We explicitly set this to NONE, because we test Server Side Encryption in a separate Context.
				s3Config.SSEKMSKeyID = ""
			})

			itCanPutAndGetAResourceThere()

			Context("With non-existing bucket", func() {
				BeforeEach(func() { s3Config.Bucket += "non-existing" })

				ItDoesNotReturnNotFoundError()
			})

			Context("With signature version 2", func() {
				BeforeEach(func() {
					if s3Config.Host != "" {
						Skip("Not on AWS")
					}
					s3Config.SignatureVersion = 2
				})

				itCanPutAndGetAResourceThere()
			})
		})

		Context("With Server Side Encryption", func() {
			var s3Client *s3sdk.S3

			JustBeforeEach(func() {
				s3Client = s3sdk.New(session.Must(session.NewSession(&aws.Config{
					Region:      aws.String(s3Config.Region),
					Credentials: credentials.NewStaticCredentials(s3Config.AccessKeyID, s3Config.SecretAccessKey, ""),
				})))
			})

			Context("AES256", func() {
				BeforeEach(func() { s3Config.ServerSideEncryption = s3.AES256 })

				It("puts the object with AES256 encryption", func() {
					if s3Config.Host != "" {
						Skip("Not on AWS")
					}
					if s3Config.SignatureVersion == 2 {
						Skip("Server side encryption does not work with signature version 2")
					}

					Expect(blobstore.Put(filepath, strings.NewReader("the file content"))).To(Succeed())

					object, e := s3Client.GetObject(&s3sdk.GetObjectInput{
						Bucket: &s3Config.Bucket,
						Key:    &filepath,
					})
					Expect(e).NotTo(HaveOccurred())
					Expect(object.ServerSideEncryption).To(Equal(aws.String(s3.AES256)))
				})
			})

			Context("aws:kms", func() {
				BeforeEach(func() { s3Config.ServerSideEncryption = s3.AWSKMS })

				It("puts the object with aws:kms encryption", func() {
					if s3Config.Host != "" {
						Skip("Not on AWS")
					}

					Expect(blobstore.Put(filepath, strings.NewReader("the file content"))).To(Succeed())

					object, e := s3Client.GetObject(&s3sdk.GetObjectInput{
						Bucket: &s3Config.Bucket,
						Key:    &filepath,
					})
					Expect(e).NotTo(HaveOccurred())
					Expect(object.ServerSideEncryption).To(Equal(aws.String(s3.AWSKMS)))
					Expect(object.SSEKMSKeyId).To(Equal(aws.String(s3Config.SSEKMSKeyID)))
				})

				It("copies the object with aws:kms encryption", func() {
					if s3Config.Host != "" {
						Skip("Not on AWS")
					}

					Expect(blobstore.Put(filepath, strings.NewReader("the file content"))).To(Succeed())
					Expect(blobstore.Copy(filepath, filepath+"_copy")).To(Succeed())

					object, e := s3Client.GetObject(&s3sdk.GetObjectInput{
						Bucket: &s3Config.Bucket,
						Key:    aws.String(filepath + "_copy"),
					})
					Expect(e).NotTo(HaveOccurred())
					Expect(object.ServerSideEncryption).To(Equal(aws.String(s3.AWSKMS)))
					Expect(object.SSEKMSKeyId).To(Equal(aws.String(s3Config.SSEKMSKeyID)))
				})
			})
		})
	})

	Context("GCP", func() {
		var gcpConfig config.GCPBlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &gcpConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = gcp.NewBlobstore(gcpConfig) })

		itCanPutAndGetAResourceThere()

		Context("With non-existing bucket", func() {
			BeforeEach(func() { gcpConfig.Bucket += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})
	})

	Context("azure", func() {
		var azureConfig config.AzureBlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &azureConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = azure.NewBlobstore(azureConfig, NewMockMetricsService()) })

		itCanPutAndGetAResourceThere()

		Context("With non-existing bucket", func() {
			BeforeEach(func() { azureConfig.ContainerName += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})
	})

	Context("openstack", func() {
		var openstackConfig config.OpenstackBlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &openstackConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = openstack.NewBlobstore(openstackConfig) })

		itCanPutAndGetAResourceThere()

		Context("With non-existing bucket", func() {
			BeforeEach(func() { openstackConfig.ContainerName += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})
	})

	Context("alibaba", func() {
		var alibabaConfig config.AlibabaBlobstoreConfig
		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &alibabaConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = alibaba.NewBlobstore(alibabaConfig) })

		itCanPutAndGetAResourceThere()

		Context("With non-existing bucket", func() {
			BeforeEach(func() { alibabaConfig.BucketName += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})
	})
})

func HaveBodyWithSubstring(substring string) types.GomegaMatcher {
	return WithTransform(func(response *http.Response) string {
		actualBytes, e := ioutil.ReadAll(response.Body)
		if e != nil {
			panic(e)
		}
		response.Body.Close()
		return string(actualBytes)
	}, Equal(substring))
}

func HaveStatusCode(statusCode int) types.GomegaMatcher {
	return WithTransform(func(response *http.Response) int {
		return response.StatusCode
	}, Equal(statusCode))
}
