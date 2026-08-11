package llm

type TransformOptions struct {
	// ArrayInstructions specifies whether the system instructions is an array.
	ArrayInstructions *bool `json:"array_instructions,omitempty"`

	// ArrayInputs specifies whether the inputs is an array.
	ArrayInputs *bool `json:"array_inputs,omitempty"`

	// DowngradeMidConversationSystem specifies whether mid-conversation system messages
	// are downgraded to user. OpenAI-compatible upstreams can hoist all system messages
	// to the prompt prefix, so downgrading later messages keeps that prefix cache-stable.
	// true = enabled, nil/false = disabled (default).
	DowngradeMidConversationSystem *bool `json:"downgrade_mid_conversation_system,omitempty"`
}
