package startup

// Toggle switches auto-start on/off
func Toggle() (enabled bool, err error) {
	if IsEnabled() {
		err = Disable()
		return false, err
	}
	err = Enable()
	return true, err
}
