package generators

// Option is a functional option for configuring generation requests.
type Option func(*Config)

// Config holds configuration for a single generation request.
// Zero-value fields mean "use the provider's default".
type Config struct {
	Model             string
	Temperature       float32
	MaxOutputTokens   int
	TopP              float32
	TopK              float32
	SystemInstruction string
	StopSequences     []string
}

// newConfig applies the given options to a zero-value Config and returns it.
func newConfig(opts []Option) *Config {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// WithModel sets the model for the generation request.
func WithModel(model string) Option {
	return func(c *Config) { c.Model = model }
}

// WithTemperature sets the sampling temperature. Higher values increase randomness.
func WithTemperature(t float32) Option {
	return func(c *Config) { c.Temperature = t }
}

// WithMaxOutputTokens sets the maximum number of tokens in the response.
func WithMaxOutputTokens(n int) Option {
	return func(c *Config) { c.MaxOutputTokens = n }
}

// WithTopP sets the nucleus sampling probability threshold.
func WithTopP(p float32) Option {
	return func(c *Config) { c.TopP = p }
}

// WithTopK sets the top-K sampling threshold.
func WithTopK(k float32) Option {
	return func(c *Config) { c.TopK = k }
}

// WithSystemInstruction sets the system-level instruction for the generation.
func WithSystemInstruction(instruction string) Option {
	return func(c *Config) { c.SystemInstruction = instruction }
}

// WithStopSequences sets sequences where generation should stop.
func WithStopSequences(seqs ...string) Option {
	return func(c *Config) { c.StopSequences = seqs }
}
