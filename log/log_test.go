package log

import (
	"fmt"
	"os"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogging(t *testing.T) {
	os.Setenv("ROLLBAR_TOKEN", "foo")
	os.Setenv("PACKET_ENV", "test")
	os.Setenv("PACKET_VERSION", "1")
	os.Setenv("ROLLBAR_DISABLE", "1")

	tests := []struct {
		level    zapcore.Level
		levels   []zapcore.Level
		messages []string
	}{
		{zap.DebugLevel, []zapcore.Level{zap.DebugLevel, zap.InfoLevel, zap.ErrorLevel}, []string{"debug", "info", "error"}},
		{zap.InfoLevel, []zapcore.Level{zap.InfoLevel, zap.ErrorLevel}, []string{"info", "error"}},
		{zap.ErrorLevel, []zapcore.Level{zap.ErrorLevel}, []string{"error"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.level), func(t *testing.T) {
			enabler := zap.NewAtomicLevelAt(tt.level)
			core, logs := observer.New(enabler)

			service := fmt.Sprintf("testing-%v", tt.level)
			logger, clean, err := configureLogger(zap.New(core), service)
			defer clean()

			if err != nil {
				t.Fatal(err)
			}

			logger.Debug("debug")
			logger.Info("info")
			logger.Error(fmt.Errorf("an error"), "error")

			if logs.Len() != len(tt.messages) {
				t.Fatalf("unexpected number of messages: want=%d, got=%d", len(tt.messages), logs.Len())
			}

			for i, log := range logs.All() {
				if log.Level != tt.levels[i] {
					t.Fatalf("unexpected log level: want=%v, got=%v", tt.levels[i], log.Level)
				}

				msg := "[" + tt.messages[i] + "]"
				got := log.Message
				if got != msg {
					t.Fatalf("unexpected message: want=%s, got=%s", msg, got)
				}

				contexts := map[string]string{
					"service": service,
				}
				if log.Level == zap.ErrorLevel {
					contexts["error"] = "an error"
				}

				ctx := log.ContextMap()
				if len(ctx) != len(contexts) {
					t.Fatalf("unexpected number of contexts: want=%d, got=%d", len(contexts), len(ctx))
				}

				for k, wantV := range contexts {
					gotV, ok := ctx[k]
					if !ok {
						t.Fatalf("missing key in context: key=%s contexts:%v", k, ctx)
					}
					if gotV != wantV {
						t.Fatalf("unexpected value for service context: want=%s, got=%s", wantV, gotV)
					}
				}
			}
		})
	}
}

func TestContext(t *testing.T) {
	os.Setenv("ROLLBAR_TOKEN", "foo")
	os.Setenv("PACKET_ENV", "test")
	os.Setenv("PACKET_VERSION", "1")
	os.Setenv("ROLLBAR_DISABLE", "1")

	enabler := zap.NewAtomicLevelAt(zap.InfoLevel)
	core, logs := observer.New(enabler)

	service := fmt.Sprintf("testing-%v", zap.InfoLevel)
	logger1, clean, err := configureLogger(zap.New(core), service)
	defer clean()

	if err != nil {
		t.Fatal(err)
	}

	assertMapsEqual := func(want, got map[string]interface{}) {
		if len(want) != len(got) {
			t.Fatalf("unexpected number of contexts: want=%d, got=%d", len(want), len(got))
		}
		for k := range want {
			vW := want[k]
			vG, ok := got[k]
			if !ok {
				t.Fatalf("missing key in context: key=%s contexts:%v", k, got)
			}
			if vW != vG {
				t.Fatalf("unexpected value for service context: want=%s, got=%s", vW, vG)
			}
		}
	}

	contexts := map[string]interface{}{
		"service": service,
	}

	logger1.Info("logger1 info")
	msgs := logs.All()

	want := 1
	if len(msgs) != want {
		t.Fatalf("unexpected number of messages: want=%d, got=%d", want, len(msgs))
	}

	assertMapsEqual(contexts, msgs[0].ContextMap())

	logger2 := logger1.With("foo", "bar")
	logger1.Info("logger1 info2")
	logger2.Info("logger2 info")
	logger1.Package("logger1").Info("packaged1 info")
	logger2.Package("logger2").Info("packaged2 info")
	logger1.Info("logger1 info3")

	msgs = logs.All()
	want = 6
	if len(msgs) != want {
		t.Fatalf("unexpected number of messages: want=%d, got=%d", want, len(msgs))
	}

	assertMapsEqual(contexts, msgs[0].ContextMap()) // hasn't changed
	assertMapsEqual(contexts, msgs[1].ContextMap())
	contexts["foo"] = "bar"
	assertMapsEqual(contexts, msgs[2].ContextMap())
	delete(contexts, "foo")
	contexts["pkg"] = "logger1"
	assertMapsEqual(contexts, msgs[3].ContextMap())
	contexts["foo"] = "bar"
	contexts["pkg"] = "logger2"
	assertMapsEqual(contexts, msgs[4].ContextMap())
	delete(contexts, "foo")
	delete(contexts, "pkg")
	assertMapsEqual(contexts, msgs[5].ContextMap())

	for i, msg := range []string{"logger1 info", "logger1 info2", "logger2 info", "packaged1 info", "packaged2 info", "logger1 info3"} {
		msg = "[" + msg + "]"
		got := msgs[i].Message
		if got != msg {
			t.Fatalf("unexpected message: want=%s, got=%s", msg, got)
		}
	}
}

func TestInit(t *testing.T) {
	os.Setenv("LOG_DISCARD_LOGS", "true")
	os.Setenv("ROLLBAR_TOKEN", "foo")
	os.Setenv("PACKET_ENV", "test")
	os.Setenv("PACKET_VERSION", "1")
	os.Setenv("ROLLBAR_DISABLE", "1")
	Init("non-debug")

	os.Setenv("DEBUG", "1")
	Init("debug")

	for _, env := range []string{"ROLLBAR_TOKEN", "PACKET_ENV", "PACKET_VERSION"} {
		t.Run(env, func(t *testing.T) {
			old := os.Getenv(env)
			os.Unsetenv(env)
			defer func() {
				os.Setenv(env, old)
				recover()
			}()
			Init("should-fail")
			t.Fatalf("should not have made it this far")
		})
	}

}