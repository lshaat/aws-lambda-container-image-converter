package extract

import (
	"archive/zip"
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/awslabs/aws-lambda-container-image-converter/img2lambda/internal/testing/mocks"
	"github.com/awslabs/aws-lambda-container-image-converter/img2lambda/types"
	imgtypes "github.com/containers/image/types"
	"github.com/golang/mock/gomock"
	"github.com/mholt/archiver"
	godigest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
)

func createImageLayer(t *testing.T,
	rawSource *mocks.MockImageSource,
	filename string,
	fileContents string,
	digest string) *imgtypes.BlobInfo {

	tar := archiver.NewTar()

	layerFile, err := ioutil.TempFile("", "")
	assert.Nil(t, err)
	_, err = layerFile.WriteString(fileContents)
	assert.Nil(t, err)
	err = layerFile.Close()
	assert.Nil(t, err)
	layerFileInfo, err := os.Stat(layerFile.Name())
	assert.Nil(t, err)

	var tarContents bytes.Buffer
	bufWriter := bufio.NewWriter(&tarContents)
	layerFile, err = os.Open(layerFile.Name())
	assert.Nil(t, err)
	err = tar.Create(bufWriter)
	assert.Nil(t, err)
	err = tar.Write(archiver.File{
		FileInfo: archiver.FileInfo{
			FileInfo:   layerFileInfo,
			CustomName: filename,
		},
		ReadCloser: layerFile,
	})
	assert.Nil(t, err)
	err = bufWriter.Flush()
	assert.Nil(t, err)
	err = tar.Close()
	assert.Nil(t, err)
	err = layerFile.Close()
	assert.Nil(t, err)
	err = os.Remove(layerFile.Name())
	assert.Nil(t, err)

	blobInfo := imgtypes.BlobInfo{Digest: godigest.Digest(digest)}

	rawSource.EXPECT().GetBlob(gomock.Any(),
		blobInfo,
		gomock.Any()).Return(ioutil.NopCloser(bytes.NewReader(tarContents.Bytes())), int64(0), nil)

	return &blobInfo
}

func validateLambdaLayer(t *testing.T,
	layer *types.LambdaLayer,
	expectedFilename string,
	expectedFileContents string,
	expectedDigest string) {

	assert.Equal(t, expectedDigest, layer.Digest)

	z := archiver.NewZip()

	zipFile, err := os.Open(layer.File)
	assert.Nil(t, err)
	zipFileInfo, err := os.Stat(zipFile.Name())
	assert.Nil(t, err)

	err = z.Open(zipFile, zipFileInfo.Size())
	assert.Nil(t, err)

	contentsFile, err := z.Read()
	assert.Nil(t, err)
	zfh, ok := contentsFile.Header.(zip.FileHeader)
	assert.True(t, ok)
	assert.Equal(t, expectedFilename, zfh.Name)

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(contentsFile.ReadCloser)
	assert.Nil(t, err)
	contents := buf.String()
	assert.Equal(t, expectedFileContents, contents)

	_, err = z.Read()
	assert.NotNil(t, err)
	assert.Equal(t, io.EOF, err)

	err = z.Close()
	assert.Nil(t, err)
	err = zipFile.Close()
	assert.Nil(t, err)
	err = os.Remove(zipFile.Name())
	assert.Nil(t, err)
}

func TestRepack(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	source := mocks.NewMockImageCloser(ctrl)
	rawSource := mocks.NewMockImageSource(ctrl)

	// Create layer tar files
	var blobInfos []imgtypes.BlobInfo

	// First matching file
	blobInfo1 := createImageLayer(t, rawSource, "opt/file1", "hello world 1", "digest1")
	blobInfos = append(blobInfos, *blobInfo1)

	// Second matching file
	blobInfo2 := createImageLayer(t, rawSource, "opt/hello/file2", "hello world 2", "digest2")
	blobInfos = append(blobInfos, *blobInfo2)

	// Irrelevant file
	blobInfo3 := createImageLayer(t, rawSource, "local/hello", "hello world 3", "digest3")
	blobInfos = append(blobInfos, *blobInfo3)

	// Overwriting previous file
	blobInfo4 := createImageLayer(t, rawSource, "opt/file1", "hello world 4", "digest4")
	blobInfos = append(blobInfos, *blobInfo4)

	source.EXPECT().LayerInfos().Return(blobInfos)

	dir, err := ioutil.TempDir("", "")
	assert.Nil(t, err)

	layers, err := repackImage(&repackOptions{
		ctx:            nil,
		cache:          nil,
		imageSource:    source,
		rawImageSource: rawSource,
		imageName:      "test-image",
		layerOutputDir: dir,
	})

	assert.Nil(t, err)
	assert.Len(t, layers, 3)

	validateLambdaLayer(t, &layers[0], "file1", "hello world 1", "digest1")
	validateLambdaLayer(t, &layers[1], "hello/file2", "hello world 2", "digest2")
	validateLambdaLayer(t, &layers[2], "file1", "hello world 4", "digest4")

	err = os.Remove(dir)
	assert.Nil(t, err)
}
