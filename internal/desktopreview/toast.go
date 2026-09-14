package desktopreview

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/capability"
)

// ToastFlow is the local presentation state for one immutable review. Native
// activation is supplied by the installed COM adapter, never a protocol URI.
// It emits an intent only after every current/saved-scope page was displayed.
// The caller still reselects and compares the current View before the common
// Session performs its own pre-decision snapshot check.
type ToastFlow struct {
	view      View
	ja        bool
	pages     []string
	page      int
	phase     string
	choice    capability.SavedChoice
	nonce     string
	displayed bool
	consumed  bool
}

type ToastChoice struct{ ID, Label string }
type ToastInput struct {
	ID, Label, Default string
	Choices            []ToastChoice
}
type ToastButton struct{ Label, Argument string }
type ToastPage struct {
	Title   string
	Body    string
	Inputs  []ToastInput
	Buttons []ToastButton
	Nonce   string
}
type ToastIntent struct {
	RequestID string
	Approved  bool
	Save      capability.SavedChoice
}

func NewToastFlow(view View, japanese bool) (*ToastFlow, error) {
	if err := ValidateView(view); err != nil {
		return nil, err
	}
	data, err := json.Marshal(view)
	if err != nil {
		return nil, err
	}
	f := &ToastFlow{ja: japanese, phase: "current"}
	if err := json.Unmarshal(data, &f.view); err != nil {
		return nil, err
	}
	f.pages, err = toastPages(struct {
		Request any    `json:"request"`
		Reason  string `json:"reason,omitempty"`
	}{view.Request.CapabilityRequest, view.Request.Reason}, japanese)
	return f, err
}

// ValidateView checks the private wire snapshot without deriving or widening
// Policy. Saved rules are returned by the common controller-owned builders.
func ValidateView(view View) error {
	if !requestPattern.MatchString(view.Request.RequestID) || !hexToken(view.Token) || !hexToken(view.Digest) || len(view.Options) > 6 {
		return ErrInvalid
	}
	data, err := json.Marshal(view.Request)
	if err != nil || len(data) > 32<<10 {
		return ErrInvalid
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != view.Digest {
		return ErrInvalid
	}
	seen := map[capability.SavedChoice]bool{}
	for _, option := range view.Options {
		kind, level, ok := strings.Cut(string(option.Choice), "-")
		if !ok || (level != "environment" && level != "global") || seen[option.Choice] {
			return ErrInvalid
		}
		decision := map[string]string{"allow": "allow", "deny": "deny", "ask": "require-approval"}[kind]
		if decision == "" || string(option.Scope.Decision) != decision {
			return ErrInvalid
		}
		seen[option.Choice] = true
	}
	return nil
}

func hexToken(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// Matches ignores newly issued private tokens but compares everything the user
// reviewed, including the reusable rule options and their ordering.
func (f *ToastFlow) Matches(current View) bool {
	return ValidateView(current) == nil && reflect.DeepEqual(f.view.Request, current.Request) && reflect.DeepEqual(f.view.Options, current.Options)
}

func (f *ToastFlow) RequestID() string { return f.view.Request.RequestID }
func (f *ToastFlow) word(en, ja string) string {
	if f.ja {
		return ja
	}
	return en
}

func (f *ToastFlow) Page() (ToastPage, error) {
	if f.consumed {
		return ToastPage{}, ErrInvalid
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return ToastPage{}, err
	}
	f.nonce = hex.EncodeToString(random[:])
	f.displayed = false
	p := ToastPage{Nonce: f.nonce}
	button := func(en, ja, action string) {
		p.Buttons = append(p.Buttons, ToastButton{f.word(en, ja), f.nonce + ":" + action})
	}
	switch f.phase {
	case "current", "scope":
		heading := f.word("Review operation", "今回の操作を確認")
		if f.phase == "scope" {
			heading = f.word("Exact saved conditions", "保存する正確な条件")
		}
		p.Title = fmt.Sprintf("%s (%d/%d)", heading, f.page+1, len(f.pages))
		p.Body = f.pages[f.page]
		if f.page > 0 {
			button("Previous", "前へ", "previous")
		}
		button("Next", "次へ", "next")
	case "choose":
		p.Title = f.word("Later matching operations", "今後の一致する操作")
		p.Body = f.word("Choose whether to save Policy.\nThis Env: only this creation.\nAll Envs: future creations too.", "設定の保存範囲を選びます。\nこのEnv: 作り直すと無効。\n全Env: 今後の作成分も対象。")
		levels := []ToastChoice{{"once", f.word("Only this operation", "今回のみ・保存しない")}}
		for _, level := range []string{"environment", "global"} {
			for _, option := range f.view.Options {
				if strings.HasSuffix(string(option.Choice), "-"+level) {
					label := f.word("This Env creation", "このEnvの今回の作成分")
					if level == "global" {
						label = f.word("All Envs, including future", "今後作成するものを含む全Env")
					}
					levels = append(levels, ToastChoice{level, label})
					break
				}
			}
		}
		p.Inputs = []ToastInput{
			{ID: "scope", Label: f.word("Scope", "保存範囲"), Default: "once", Choices: levels},
			{ID: "policy", Label: f.word("Saved decision (when saving)", "保存する場合の判断"), Default: "ask", Choices: []ToastChoice{{"ask", f.word("Ask each time", "毎回確認")}, {"allow", f.word("Allow matching operations", "一致する操作を許可")}, {"deny", f.word("Deny matching operations", "一致する操作を拒否")}}},
		}
		button("Review this choice", "選んだ設定を確認", "choose")
	case "confirm":
		p.Title = f.word("Answer this operation", "今回の操作に回答")
		p.Body = f.word("All displayed conditions apply.\nNo Policy will be saved.", "確認した今回の操作に回答。\n設定は保存しません。")
		if f.choice != "" {
			level := f.word("this Env creation", "このEnvの今回の作成分")
			if strings.HasSuffix(string(f.choice), "-global") {
				level = f.word("all Envs, including future", "今後作成するものを含む全Env")
			}
			kind := strings.SplitN(string(f.choice), "-", 2)[0]
			label := map[string]string{"allow": f.word("allow", "許可"), "deny": f.word("deny", "拒否"), "ask": f.word("ask each time", "毎回確認")}[kind]
			p.Body = f.word("Save: ", "保存する判断: ") + label + "\n" + level + "\n" + f.word("Then answer the current operation.", "今回の操作に回答してください。")
		}
		if !strings.HasPrefix(string(f.choice), "allow-") {
			button("Deny this operation", "今回は拒否", "deny")
		}
		if !strings.HasPrefix(string(f.choice), "deny-") {
			button("Allow this operation", "今回は許可", "allow")
		}
		button("Change saved choice", "保存の選択へ戻る", "change")
	default:
		return ToastPage{}, ErrInvalid
	}
	return p, nil
}

// Displayed is called only after the native Show operation succeeded. Rendering
// an XML string alone is insufficient to enable navigation or an answer.
func (f *ToastFlow) Displayed(nonce string) error {
	if f.consumed || nonce == "" || nonce != f.nonce {
		return ErrInvalid
	}
	f.displayed = true
	return nil
}

func (f *ToastFlow) Activate(argument string, inputs map[string]string) (*ToastIntent, error) {
	nonce, action, ok := strings.Cut(argument, ":")
	if !ok || f.consumed || !f.displayed || nonce != f.nonce {
		return nil, ErrInvalid
	}
	// Every activation consumes its displayed page before a fallible operation.
	f.displayed = false
	f.nonce = ""
	if action != "choose" && len(inputs) != 0 {
		return nil, ErrInvalid
	}
	switch {
	case (f.phase == "current" || f.phase == "scope") && action == "previous" && f.page > 0:
		f.page--
	case (f.phase == "current" || f.phase == "scope") && action == "next":
		f.page++
		if f.page == len(f.pages) {
			if f.phase == "current" {
				f.phase = "choose"
			} else {
				f.phase = "confirm"
			}
			f.page = 0
		}
	case f.phase == "choose" && action == "choose":
		if len(inputs) != 2 || (inputs["policy"] != "ask" && inputs["policy"] != "allow" && inputs["policy"] != "deny") {
			return nil, ErrInvalid
		}
		f.choice = ""
		if inputs["scope"] == "once" {
			f.phase = "confirm"
			return nil, nil
		}
		choice := capability.SavedChoice(inputs["policy"] + "-" + inputs["scope"])
		for _, option := range f.view.Options {
			if option.Choice == choice {
				pages, err := toastPages(option.Scope, f.ja)
				if err != nil {
					return nil, err
				}
				f.choice, f.pages, f.page, f.phase = choice, pages, 0, "scope"
				return nil, nil
			}
		}
		return nil, ErrInvalid
	case f.phase == "confirm" && action == "change":
		f.phase = "choose"
		f.choice = ""
	case f.phase == "confirm" && (action == "allow" || action == "deny"):
		approved := action == "allow"
		if (approved && strings.HasPrefix(string(f.choice), "deny-")) || (!approved && strings.HasPrefix(string(f.choice), "allow-")) {
			return nil, ErrInvalid
		}
		f.consumed = true
		return &ToastIntent{f.view.Request.RequestID, approved, f.choice}, nil
	default:
		return nil, ErrInvalid
	}
	return nil, nil
}

// Each page is four explicit, at-most-34-cell lines. Values are never shortened:
// continuation pages preserve even long refs, paths and exact saved constraints.
func toastPages(value any, ja bool) ([]string, error) {
	data, err := json.Marshal(value)
	if err != nil || len(data) > 64<<10 {
		return nil, ErrInvalid
	}
	var object any
	if json.Unmarshal(data, &object) != nil {
		return nil, ErrInvalid
	}
	var lines []string
	var visit func(string, any)
	visit = func(key string, v any) {
		if fields, ok := v.(map[string]any); ok {
			keys := make([]string, 0, len(fields))
			for k := range fields {
				keys = append(keys, k)
			}
			rank := func(k string) int {
				v, ok := map[string]int{"request": 0, "environment": 0, "capability": 1, "action": 2, "resource": 3, "environment_instance": 4, "attributes": 5, "decision": 6, "reason": 9}[k]
				if !ok {
					return 8
				}
				return v
			}
			sort.Slice(keys, func(i, j int) bool {
				a, b := rank(keys[i]), rank(keys[j])
				if a == b {
					return keys[i] < keys[j]
				}
				return a < b
			})
			for _, k := range keys {
				label := k
				if key != "" {
					label = key + "." + k
				}
				visit(label, fields[k])
			}
			return
		}
		encoded, _ := json.Marshal(v)
		text := string(encoded)
		if s, ok := v.(string); ok {
			text = s
		}
		key = strings.TrimPrefix(key, "request.")
		label := toastFieldLabel(key, ja)
		lines = append(lines, wrapToastText(label+": "+printableToast(text), 34)...)
	}
	visit("", object)
	if len(lines) == 0 {
		return nil, errors.New("empty review")
	}
	pages := make([]string, 0, (len(lines)+3)/4)
	for len(lines) > 0 {
		n := min(4, len(lines))
		pages = append(pages, strings.Join(lines[:n], "\n"))
		lines = lines[n:]
	}
	return pages, nil
}

func toastFieldLabel(key string, ja bool) string {
	labels := map[string][2]string{"capability": {"Capability", "機能"}, "action": {"Action", "操作"}, "resource": {"Target", "対象"}, "environment": {"Env", "Env"}, "environment_instance": {"Env creation", "Env作成識別子"}, "reason": {"Reason", "理由"}, "decision": {"Saved decision", "保存する判断"}, "expires_at": {"Expiry", "有効期限"}}
	if label, ok := labels[key]; ok {
		if ja {
			return label[1]
		}
		return label[0]
	}
	return printableToast(key)
}

func printableToast(value string) string {
	var out strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			fmt.Fprintf(&out, "\\u%04x", r)
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func wrapToastText(value string, width int) []string {
	var result []string
	start, cells := 0, 0
	for index, r := range value {
		w := 1
		if r > 127 {
			w = 2
		}
		if cells+w > width {
			result = append(result, value[start:index])
			start = index
			cells = 0
		}
		cells += w
	}
	if start < len(value) {
		result = append(result, value[start:])
	}
	if !utf8.ValidString(value) {
		return []string{"[invalid text]"}
	}
	return result
}
