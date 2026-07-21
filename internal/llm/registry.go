package llm

type Capabilities struct {
	ToolCalling bool
}

type ProviderDescriptor struct {
	ID           string
	DisplayName  string
	DefaultModel string
	DefaultURL   string
	Capabilities Capabilities
}

var providers = []ProviderDescriptor{
	{
		ID:           "volcengine",
		DisplayName:  "火山引擎方舟",
		DefaultURL:   "https://ark.cn-beijing.volces.com/api/v3",
		Capabilities: Capabilities{ToolCalling: true},
	},
}

func Providers() []ProviderDescriptor {
	result := make([]ProviderDescriptor, len(providers))
	copy(result, providers)
	return result
}

func Provider(id string) (ProviderDescriptor, bool) {
	for _, provider := range providers {
		if provider.ID == id {
			return provider, true
		}
	}
	return ProviderDescriptor{}, false
}
