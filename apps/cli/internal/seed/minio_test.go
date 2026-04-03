package seed

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type DetectContentTypeTestSuite struct {
	suite.Suite
}

func TestDetectContentTypeTestSuite(t *testing.T) {
	suite.Run(t, new(DetectContentTypeTestSuite))
}

func (s *DetectContentTypeTestSuite) Test_json() {
	s.Assert().Equal("application/json", DetectContentType("data.json", nil))
}

func (s *DetectContentTypeTestSuite) Test_yaml() {
	s.Assert().Equal("application/x-yaml", DetectContentType("config.yaml", nil))
}

func (s *DetectContentTypeTestSuite) Test_yml() {
	s.Assert().Equal("application/x-yaml", DetectContentType("config.yml", nil))
}

func (s *DetectContentTypeTestSuite) Test_png() {
	s.Assert().Equal("image/png", DetectContentType("logo.png", nil))
}

func (s *DetectContentTypeTestSuite) Test_jpg() {
	s.Assert().Equal("image/jpeg", DetectContentType("photo.jpg", nil))
}

func (s *DetectContentTypeTestSuite) Test_jpeg() {
	s.Assert().Equal("image/jpeg", DetectContentType("photo.jpeg", nil))
}

func (s *DetectContentTypeTestSuite) Test_gif() {
	s.Assert().Equal("image/gif", DetectContentType("anim.gif", nil))
}

func (s *DetectContentTypeTestSuite) Test_svg() {
	s.Assert().Equal("image/svg+xml", DetectContentType("icon.svg", nil))
}

func (s *DetectContentTypeTestSuite) Test_html() {
	s.Assert().Equal("text/html", DetectContentType("page.html", nil))
}

func (s *DetectContentTypeTestSuite) Test_css() {
	s.Assert().Equal("text/css", DetectContentType("style.css", nil))
}

func (s *DetectContentTypeTestSuite) Test_js() {
	s.Assert().Equal("application/javascript", DetectContentType("app.js", nil))
}

func (s *DetectContentTypeTestSuite) Test_txt() {
	s.Assert().Equal("text/plain", DetectContentType("readme.txt", nil))
}

func (s *DetectContentTypeTestSuite) Test_unknown_extension_falls_back_to_content_sniffing() {
	// http.DetectContentType returns "text/plain; charset=utf-8" for plain text.
	result := DetectContentType("file.xyz", []byte("hello world"))
	s.Assert().Contains(result, "text/plain")
}

func (s *DetectContentTypeTestSuite) Test_unknown_extension_binary_content() {
	// http.DetectContentType returns "application/octet-stream" for unrecognised binary.
	result := DetectContentType("file.bin", []byte{0x00, 0x01, 0x02, 0xFF})
	s.Assert().Equal("application/octet-stream", result)
}
