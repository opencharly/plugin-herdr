package herdr

import (
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/opencharly/sdk"
)

func TestCommandModel(t *testing.T) {
	model, err := commandModel()
	if err != nil {
		t.Fatalf("commandModel() = %v", err)
	}
	if model.Name != "herdr" {
		t.Fatalf("commandModel().Name = %q, want herdr", model.Name)
	}
}

func TestRunInProcCLI_Help(t *testing.T) {
	var command HerdrCmd
	err := sdk.RunInProcCLI("herdr", &command, []string{"--help"},
		kong.Description("test"))
	if err != nil {
		t.Fatalf("RunInProcCLI(--help) = %v, want nil", err)
	}
}

func TestRunInProcCLI_ConfigWithSession(t *testing.T) {
	var command HerdrCmd
	err := sdk.RunInProcCLI("herdr", &command, []string{"config", "--session", "spike"},
		kong.Description("test"))
	if err != nil {
		t.Fatalf("RunInProcCLI(config --session spike) = %v, want nil", err)
	}
}

func TestRunInProcCLI_ConfigRefusesFocusedByDefault(t *testing.T) {
	// Inside a herdr session the env lifts the guard, so force the
	// outside-herdr state for this test.
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERDR_ENV", "0")
	var command HerdrCmd
	err := sdk.RunInProcCLI("herdr", &command, []string{"config"},
		kong.Description("test"))
	if err == nil {
		t.Fatal("config without a target = nil, want the focused-session guard error")
	}
	if !strings.Contains(err.Error(), "focused session is off-limits") {
		t.Fatalf("err = %v, want focused-session guard", err)
	}
}

func TestRunInProcCLI_UnknownCommand(t *testing.T) {
	var command HerdrCmd
	err := sdk.RunInProcCLI("herdr", &command, []string{"bogus"},
		kong.Description("test"))
	if err == nil {
		t.Fatal("RunInProcCLI(bogus) = nil, want parse error")
	}
}
