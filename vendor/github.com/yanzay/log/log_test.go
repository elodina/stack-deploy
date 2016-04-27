package log

import (
	"fmt"
	"testing"

	"flag"
	"os"

	c "github.com/smartystreets/goconvey/convey"
)

type MockWriter struct {
	lastLog []byte
}

func (mw *MockWriter) Write(v []byte) (int, error) {
	mw.lastLog = v
	return len(v), nil
}

func (mw *MockWriter) GetLastLog() string {
	return string(mw.lastLog)
}

func TestPrintln(t *testing.T) {
	c.Convey("Given a MockWriter", t, func() {
		writer := &MockWriter{}
		Writer = writer
		Println("test")
		c.So(writer.GetLastLog(), c.ShouldEqual, "test")
	})
}

func TestTrace(t *testing.T) {
	c.Convey("Given a MockWriter", t, func() {
		writer := &MockWriter{}
		Writer = writer
		c.Convey("When Level is LevelTrace", func() {
			Level = LevelTrace
			Trace("test")
			c.So(writer.GetLastLog(), c.ShouldEqual, "[TRACE] test")
		})
		c.Convey("When Level is LevelDebug", func() {
			Level = LevelDebug
			Trace("badtest")
			c.So(writer.GetLastLog(), c.ShouldNotEqual, "[DEBUG] badtest")
		})
	})
}

func TestDebug(t *testing.T) {
	c.Convey("Given a MockWriter", t, func() {
		writer := &MockWriter{}
		Writer = writer
		c.Convey("When Level is LevelDebug", func() {
			Level = LevelDebug
			Debug("test")
			c.So(writer.GetLastLog(), c.ShouldEqual, "[DEBUG] test")
		})
		c.Convey("When Level is LevelInfo", func() {
			Level = LevelInfo
			Debug("badtest")
			c.So(writer.GetLastLog(), c.ShouldNotEqual, "[INFO] badtest")
		})
	})
}

func TestInfo(t *testing.T) {
	c.Convey("Given a MockWriter", t, func() {
		writer := &MockWriter{}
		Writer = writer
		c.Convey("When Level is LevelInfo", func() {
			Level = LevelInfo
			Info("test")
			c.So(writer.GetLastLog(), c.ShouldEqual, "[INFO] test")
		})
		c.Convey("Without Level setting", func() {
			Level = 0
			Info("test")
			c.So(writer.GetLastLog(), c.ShouldEqual, "[INFO] test")
		})
		c.Convey("When Level is LevelWarning", func() {
			Level = LevelWarning
			Info("badtest")
			c.So(writer.GetLastLog(), c.ShouldNotEqual, "[WARNING] badtest")
		})
	})
}

func TestWarning(t *testing.T) {
	c.Convey("Given a MockWriter", t, func() {
		writer := &MockWriter{}
		Writer = writer
		c.Convey("When Level is LevelWarning", func() {
			Level = LevelWarning
			Warning("test")
			c.So(writer.GetLastLog(), c.ShouldEqual, "[WARNING] test")
		})
		c.Convey("When Level is LevelError", func() {
			Level = LevelError
			Warning("badtest")
			c.So(writer.GetLastLog(), c.ShouldNotEqual, "[ERROR] badtest")
		})
	})
}

func TestError(t *testing.T) {
	c.Convey("Given a MockWriter", t, func() {
		writer := &MockWriter{}
		Writer = writer
		c.Convey("When Level is LevelError", func() {
			Level = LevelError
			Error("test")
			c.So(writer.GetLastLog(), c.ShouldEqual, "[ERROR] test")
		})
		c.Convey("When Level is LevelFatal", func() {
			Level = LevelFatal
			Error("badtest")
			c.So(writer.GetLastLog(), c.ShouldNotEqual, "[FATAL] badtest")
		})
	})
}

func TestFatal(t *testing.T) {
	c.Convey("Given a MockWriter", t, func() {
		writer := &MockWriter{}
		Writer = writer
		c.Convey("When Level is LevelFatal", func() {
			Level = LevelFatal
			c.So(func() { Fatal("fatal") }, c.ShouldPanic)
			c.So(writer.GetLastLog(), c.ShouldEqual, "[FATAL] fatal")
		})
	})
}

func TestFlags(t *testing.T) {
	c.Convey("Flags should be parsed and translated to log levels", t, func() {
		c.Convey("When flag is 'trace'", func() {
			os.Args = []string{"", "--log-level", "trace"}
			flag.Parse()
			c.So(Level.String(), c.ShouldEqual, "trace")
			c.So(Level, c.ShouldEqual, LevelTrace)
		})
		c.Convey("When flag is 'debug'", func() {
			os.Args = []string{"", "--log-level", "debug"}
			flag.Parse()
			c.So(Level.String(), c.ShouldEqual, "debug")
			c.So(Level, c.ShouldEqual, LevelDebug)
		})
		c.Convey("When flag is 'info'", func() {
			os.Args = []string{"", "--log-level", "info"}
			flag.Parse()
			c.So(Level.String(), c.ShouldEqual, "info")
			c.So(Level, c.ShouldEqual, LevelInfo)
		})
		c.Convey("When flag is 'warning'", func() {
			os.Args = []string{"", "--log-level", "warning"}
			flag.Parse()
			c.So(Level.String(), c.ShouldEqual, "warning")
			c.So(Level, c.ShouldEqual, LevelWarning)
		})
		c.Convey("When flag is 'error'", func() {
			os.Args = []string{"", "--log-level", "error"}
			flag.Parse()
			c.So(Level.String(), c.ShouldEqual, "error")
			c.So(Level, c.ShouldEqual, LevelError)
		})
		c.Convey("When flag is 'fatal'", func() {
			os.Args = []string{"", "--log-level", "fatal"}
			flag.Parse()
			c.So(Level.String(), c.ShouldEqual, "fatal")
			c.So(Level, c.ShouldEqual, LevelFatal)
		})
	})

	c.Convey("Flags should yield an error", t, func() {
		Level = LevelInfo
		c.Convey("When flag is 'incorrect'", func() {
			args := []string{"--log-level", "incorrect"}
			flags := flag.NewFlagSet("testing", flag.ContinueOnError)
			flags.Var(&Level, "log-level", "Log level: trace|debug|info|warning|error|fatal")
			err := flags.Parse(args)
			c.So(err.Error(), c.ShouldContainSubstring, "Unknown logging level incorrect")

			c.So(LogLevel(0).String(), c.ShouldEqual, "unknown")
			c.So(Level, c.ShouldEqual, LevelInfo)
		})
	})
}

func TestFormat(t *testing.T) {
	c.Convey("Given format funcs", t, func() {
		funcs := map[string]func(string, ...interface{}){
			// "Printf":   Printf,
			"Tracef":   Tracef,
			"Debugf":   Debugf,
			"Infof":    Infof,
			"Warningf": Warningf,
			"Errorf":   Errorf,
		}
		writer := &MockWriter{}
		Writer = writer
		Level = LevelTrace
		for name, fun := range funcs {
			c.Convey(name+" format should work", func() {
				fun("%s answer: %d", name, 42)
				c.So(writer.GetLastLog(), c.ShouldContainSubstring, name+" answer: 42")
			})
		}
		c.Convey("Fatalf format should work", func() {
			c.So(func() { Fatalf("answer: %d", 42) }, c.ShouldPanic)
			c.So(writer.GetLastLog(), c.ShouldContainSubstring, "answer: 42")
		})

	})
}

type BadWriter struct{}

func (bw BadWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("It's a bad writer, don't write here!")
}

func TestBadWriter(t *testing.T) {
	c.Convey("Given bad writer", t, func() {
		Writer = BadWriter{}
		c.Convey("Log should not fail", func() {
			c.So(func() { printString("hi, bad writer") }, c.ShouldNotPanic)
		})
	})
}

func TestMultiWriter(t *testing.T) {
	c.Convey("Given two writers", t, func() {
		writer1 := &MockWriter{}
		writer2 := &MockWriter{}
		Writer = nil
		AddWriter(writer1)
		AddWriter(writer2)
		Println("testmulti")
		c.So(writer1.GetLastLog(), c.ShouldEqual, "testmulti")
		c.So(writer2.GetLastLog(), c.ShouldEqual, "testmulti")
	})
}
