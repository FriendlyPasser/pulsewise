package gui

import (
	"errors"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type fakeLoginItemDriver struct {
	isAvailable bool
	liveStatus  launchStatus
	statusErr   error
	setErr      error
	nextStatus  *launchStatus
	setCalls    []bool
	openCalls   int
}

func (d *fakeLoginItemDriver) available() bool { return d.isAvailable }

func (d *fakeLoginItemDriver) status() (launchStatus, error) {
	if d.statusErr != nil {
		return launchUnavailable, d.statusErr
	}
	return d.liveStatus, nil
}

func (d *fakeLoginItemDriver) setEnabled(enabled bool) error {
	d.setCalls = append(d.setCalls, enabled)
	if d.setErr != nil {
		return d.setErr
	}
	if d.nextStatus != nil {
		d.liveStatus = *d.nextStatus
		return nil
	}
	if enabled {
		d.liveStatus = launchEnabled
	} else {
		d.liveStatus = launchDisabled
	}
	return nil
}

func (d *fakeLoginItemDriver) openSystemSettings() error {
	d.openCalls++
	return nil
}

type fakeLoginPrompt struct {
	confirmCalls int
	errors       []string
}

func (p *fakeLoginPrompt) confirmOpenSettings() {
	p.confirmCalls++
}

func (p *fakeLoginPrompt) showError(title, _ string) {
	p.errors = append(p.errors, title)
}

func TestSyncLoginItemReflectsLiveStatus(t *testing.T) {
	cases := []struct {
		name        string
		status      launchStatus
		statusErr   error
		wantChecked bool
		wantTooltip string
		wantReturn  launchStatus
	}{
		{
			name:        "disabled",
			status:      launchDisabled,
			wantChecked: false,
			wantTooltip: loginTooltipDefault,
			wantReturn:  launchDisabled,
		},
		{
			name:        "enabled",
			status:      launchEnabled,
			wantChecked: true,
			wantTooltip: loginTooltipDefault,
			wantReturn:  launchEnabled,
		},
		{
			name:        "requires approval",
			status:      launchRequiresApproval,
			wantChecked: false,
			wantTooltip: loginTooltipRequiresApproval,
			wantReturn:  launchRequiresApproval,
		},
		{
			name:        "unavailable",
			status:      launchUnavailable,
			wantChecked: false,
			wantTooltip: loginTooltipDefault,
			wantReturn:  launchUnavailable,
		},
		{
			name:        "status error clears checked state",
			status:      launchEnabled,
			statusErr:   errors.New("query failed"),
			wantChecked: false,
			wantTooltip: loginTooltipDefault,
			wantReturn:  launchUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := application.NewMenuItemCheckbox("登录时打开", true)
			driver := &fakeLoginItemDriver{
				isAvailable: true,
				liveStatus:  tc.status,
				statusErr:   tc.statusErr,
			}

			got := syncLoginItem(item, driver)
			if got != tc.wantReturn {
				t.Fatalf("syncLoginItem() = %v, want %v", got, tc.wantReturn)
			}
			if item.Checked() != tc.wantChecked {
				t.Fatalf("item.Checked() = %v, want %v", item.Checked(), tc.wantChecked)
			}
			if item.Tooltip() != tc.wantTooltip {
				t.Fatalf("item.Tooltip() = %q, want %q", item.Tooltip(), tc.wantTooltip)
			}
		})
	}

	if got := syncLoginItem(nil, &fakeLoginItemDriver{isAvailable: true, liveStatus: launchEnabled}); got != launchUnavailable {
		t.Fatalf("syncLoginItem(nil) = %v, want %v", got, launchUnavailable)
	}
}

func TestToggleLoginItemTransitions(t *testing.T) {
	item := application.NewMenuItemCheckbox("登录时打开", false)
	driver := &fakeLoginItemDriver{isAvailable: true, liveStatus: launchDisabled}
	prompt := &fakeLoginPrompt{}

	toggleLoginItem(item, driver, prompt)
	if len(driver.setCalls) != 1 || !driver.setCalls[0] {
		t.Fatalf("expected enable call, got %v", driver.setCalls)
	}
	if !item.Checked() || item.Tooltip() != loginTooltipDefault {
		t.Fatalf("after enable: checked=%v tooltip=%q", item.Checked(), item.Tooltip())
	}
	if prompt.confirmCalls != 0 || len(prompt.errors) != 0 {
		t.Fatalf("unexpected prompts after enable: %+v", prompt)
	}

	toggleLoginItem(item, driver, prompt)
	if len(driver.setCalls) != 2 || driver.setCalls[1] {
		t.Fatalf("expected disable call, got %v", driver.setCalls)
	}
	if item.Checked() || item.Tooltip() != loginTooltipDefault {
		t.Fatalf("after disable: checked=%v tooltip=%q", item.Checked(), item.Tooltip())
	}
	if prompt.confirmCalls != 0 || len(prompt.errors) != 0 {
		t.Fatalf("unexpected prompts after disable: %+v", prompt)
	}
}

func TestToggleLoginItemRequiresApproval(t *testing.T) {
	t.Run("already requires approval", func(t *testing.T) {
		item := application.NewMenuItemCheckbox("登录时打开", true)
		driver := &fakeLoginItemDriver{isAvailable: true, liveStatus: launchRequiresApproval}
		prompt := &fakeLoginPrompt{}

		toggleLoginItem(item, driver, prompt)
		if len(driver.setCalls) != 0 {
			t.Fatalf("unexpected setEnabled calls: %v", driver.setCalls)
		}
		if item.Checked() || item.Tooltip() != loginTooltipRequiresApproval {
			t.Fatalf("checked=%v tooltip=%q", item.Checked(), item.Tooltip())
		}
		if prompt.confirmCalls != 1 || len(prompt.errors) != 0 {
			t.Fatalf("unexpected prompt calls: %+v", prompt)
		}
	})

	t.Run("setEnabled returns approval error", func(t *testing.T) {
		item := application.NewMenuItemCheckbox("登录时打开", true)
		driver := &fakeLoginItemDriver{
			isAvailable: true,
			liveStatus:  launchDisabled,
			setErr:      errLaunchRequiresApproval,
		}
		prompt := &fakeLoginPrompt{}

		toggleLoginItem(item, driver, prompt)
		if item.Checked() || item.Tooltip() != loginTooltipRequiresApproval {
			t.Fatalf("checked=%v tooltip=%q", item.Checked(), item.Tooltip())
		}
		if prompt.confirmCalls != 1 || len(prompt.errors) != 0 {
			t.Fatalf("unexpected prompt calls: %+v", prompt)
		}
	})

	t.Run("live status transitions to requires approval after register", func(t *testing.T) {
		next := launchRequiresApproval
		item := application.NewMenuItemCheckbox("登录时打开", true)
		driver := &fakeLoginItemDriver{
			isAvailable: true,
			liveStatus:  launchDisabled,
			nextStatus:  &next,
		}
		prompt := &fakeLoginPrompt{}

		toggleLoginItem(item, driver, prompt)
		if item.Checked() || item.Tooltip() != loginTooltipRequiresApproval {
			t.Fatalf("checked=%v tooltip=%q", item.Checked(), item.Tooltip())
		}
		if prompt.confirmCalls != 1 || len(prompt.errors) != 0 {
			t.Fatalf("unexpected prompt calls: %+v", prompt)
		}
	})
}

func TestToggleLoginItemRestoresStateOnError(t *testing.T) {
	t.Run("enable failure restores unchecked", func(t *testing.T) {
		item := application.NewMenuItemCheckbox("登录时打开", true)
		driver := &fakeLoginItemDriver{
			isAvailable: true,
			liveStatus:  launchDisabled,
			setErr:      errors.New("invalid signature"),
		}
		prompt := &fakeLoginPrompt{}

		toggleLoginItem(item, driver, prompt)
		if item.Checked() {
			t.Fatal("expected item to be restored to unchecked")
		}
		if len(prompt.errors) != 1 || prompt.confirmCalls != 0 {
			t.Fatalf("unexpected prompt calls: %+v", prompt)
		}
	})

	t.Run("disable failure restores checked", func(t *testing.T) {
		preFlippedItem := application.NewMenuItemCheckbox("登录时打开", false)
		driver := &fakeLoginItemDriver{
			isAvailable: true,
			liveStatus:  launchEnabled,
			setErr:      errors.New("unregister failed"),
		}
		prompt := &fakeLoginPrompt{}

		toggleLoginItem(preFlippedItem, driver, prompt)
		if !preFlippedItem.Checked() {
			t.Fatal("expected item to be restored to checked")
		}
		if len(prompt.errors) != 1 || prompt.confirmCalls != 0 {
			t.Fatalf("unexpected prompt calls: %+v", prompt)
		}
	})

	t.Run("status read failure skips mutation and shows error", func(t *testing.T) {
		item := application.NewMenuItemCheckbox("登录时打开", true)
		driver := &fakeLoginItemDriver{
			isAvailable: true,
			statusErr:   errors.New("status unavailable"),
		}
		prompt := &fakeLoginPrompt{}

		toggleLoginItem(item, driver, prompt)
		if len(driver.setCalls) != 0 {
			t.Fatalf("unexpected setEnabled calls: %v", driver.setCalls)
		}
		if item.Checked() {
			t.Fatal("expected item to be unchecked when status is unreadable")
		}
		if len(prompt.errors) != 1 || prompt.confirmCalls != 0 {
			t.Fatalf("unexpected prompt calls: %+v", prompt)
		}
	})
}
