package gemini

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"

	"gemini-free/internal/generation"
)

const (
	fieldCount     = 102
	modeIndex      = 79
	metadataHeader = "X-Goog-Ext-525001261-Jspb"
)

func payload(prompt string, model generation.Model, token string) (string, error) {
	fields := make([]any, fieldCount)
	fields[0] = []any{prompt, 0, nil, nil, nil, nil, 0}
	fields[1] = []string{"en"}
	fields[2] = []any{"", "", "", nil, nil, nil, nil, nil, nil, ""}
	fields[6], fields[7] = []int{0}, 1
	fields[10], fields[11] = 1, 0
	fields[17], fields[18] = [][]int{{model.Think}}, 0
	fields[27], fields[30] = 1, []int{4}
	fields[41], fields[53] = []int{2}, 0
	fields[59], fields[61] = randomID(), []any{}
	fields[68], fields[modeIndex] = 1, model.Mode
	for index, value := range model.Extra {
		fields[index] = value
	}
	inner, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	outer, err := json.Marshal([]any{nil, string(inner)})
	if err != nil {
		return "", err
	}
	form := url.Values{"f.req": {string(outer)}}
	if token != "" {
		form.Set("at", token)
	}
	return form.Encode(), nil
}

func modelHeaders(model generation.Model) map[string]string {
	if model.Mode != generation.ArgonMode {
		return nil
	}
	data, _ := json.Marshal([]any{1, nil, nil, nil, generation.ArgonModelID, nil, nil, 0,
		[]int{4}, nil, nil, 2, nil, nil, model.Mode, model.Extra[generation.ArgonThinking]})
	return map[string]string{metadataHeader: string(data)}
}

func randomID() string {
	return hex.EncodeToString(randomBytes(16))
}

func randomBytes(size int) []byte {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
