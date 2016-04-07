package log

import (
	"testing"

	c "github.com/smartystreets/goconvey/convey"
)

func TestAsyncWrite(t *testing.T) {
	c.Convey("Given AsyncWriter", t, func() {
		Writer = NewAsyncWriter()
		c.Convey("Should return 0 bytes and no error", func() {
			count, err := Writer.Write([]byte("test"))
			c.So(count, c.ShouldBeZeroValue)
			c.So(err, c.ShouldBeNil)
		})
	})
}
