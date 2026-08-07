package biz

import "context"

type disabledTurnstileVerifier struct{}

func (disabledTurnstileVerifier) Verify(context.Context, string, string) error {
	return nil
}

func (disabledTurnstileVerifier) PublicConfig(context.Context) (*PublicTurnstileConfig, error) {
	return newPublicTurnstileConfig(&StoredTurnstileSettings{}), nil
}

type turnstileVerifyCall struct {
	token  string
	action string
}

type recordingTurnstileVerifier struct {
	calls     []turnstileVerifyCall
	verifyErr error
	config    *PublicTurnstileConfig
	configErr error
}

func (v *recordingTurnstileVerifier) Verify(_ context.Context, token, action string) error {
	v.calls = append(v.calls, turnstileVerifyCall{token: token, action: action})
	return v.verifyErr
}

func (v *recordingTurnstileVerifier) PublicConfig(context.Context) (*PublicTurnstileConfig, error) {
	if v.configErr != nil {
		return nil, v.configErr
	}
	if v.config != nil {
		return v.config, nil
	}

	return newPublicTurnstileConfig(&StoredTurnstileSettings{}), nil
}
