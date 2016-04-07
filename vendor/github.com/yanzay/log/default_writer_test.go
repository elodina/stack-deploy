package log

import (
	"testing"

	c "github.com/smartystreets/goconvey/convey"
)

func TestWrite(t *testing.T) {
	c.Convey("Given DefaultWriter", t, func() {
		Writer = DefaultWriter{}
		c.Convey("Write should go to stdout", func() {
			count, err := Writer.Write([]byte("test"))
			c.So(err, c.ShouldBeNil)
			c.So(count, c.ShouldEqual, 4)
		})
	})
}
