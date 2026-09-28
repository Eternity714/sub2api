package service

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"strconv"
	"testing"
)

type videoDownloadStub struct{ payload []byte }

func (d videoDownloadStub) Download(_ context.Context, _ string) (io.ReadCloser, int64, error) {
	return io.NopCloser(bytes.NewReader(d.payload)), int64(len(d.payload)), nil
}

type videoStorageStub struct{ keys []string }

func (s *videoStorageStub) SaveVideoReader(_ context.Context, key string, body io.Reader, size int64) (ImageStorageSaveMetadata, error) {
	b, e := io.ReadAll(body)
	if e != nil {
		return ImageStorageSaveMetadata{}, e
	}
	if int64(len(b)) != size {
		panic("size")
	}
	s.keys = append(s.keys, key)
	i := len(s.keys) - 1
	return ImageStorageSaveMetadata{URL: "https://media.example/video-" + strconv.Itoa(i) + ".mp4", ObjectKey: key, ContentType: "video/mp4"}, nil
}
func TestGrsaiVideoPersistencePreservesOrderAndStableKeys(t *testing.T) {
	s := &videoStorageStub{}
	p := NewGrsaiVideoPersister(videoDownloadStub{append([]byte("....ftypisom"), make([]byte, 1024)...)}, s)
	got, e := p.PersistGrsaiVideo(context.Background(), "grsai_test", &GrsaiUpstreamResult{Status: GrsaiUpstreamStatusSucceeded, ImageURLs: []string{"https://upstream.example/a", "https://upstream.example/b"}})
	require.NoError(t, e)
	require.JSONEq(t, `{"results":[{"url":"https://media.example/video-0.mp4"},{"url":"https://media.example/video-1.mp4"}]}`, string(got.ResultJSON))
	require.Equal(t, []string{"grsai/grsai_test/video-0.mp4", "grsai/grsai_test/video-1.mp4"}, s.keys)
}
