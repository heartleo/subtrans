package translator

// languageNames maps ISO 639-1 codes to full language names for LLM prompts.
var languageNames = map[string]string{
	"zh":    "Simplified Chinese",
	"zh-CN": "Simplified Chinese",
	"zh-TW": "Traditional Chinese",
	"en":    "English",
	"ja":    "Japanese",
	"ko":    "Korean",
	"fr":    "French",
	"de":    "German",
	"es":    "Spanish",
	"pt":    "Portuguese",
	"pt-BR": "Brazilian Portuguese",
	"ru":    "Russian",
	"ar":    "Arabic",
	"it":    "Italian",
	"nl":    "Dutch",
	"pl":    "Polish",
	"tr":    "Turkish",
	"vi":    "Vietnamese",
	"th":    "Thai",
	"id":    "Indonesian",
	"ms":    "Malay",
	"hi":    "Hindi",
	"uk":    "Ukrainian",
	"cs":    "Czech",
	"sv":    "Swedish",
	"da":    "Danish",
	"fi":    "Finnish",
	"no":    "Norwegian",
	"hu":    "Hungarian",
	"el":    "Greek",
	"ro":    "Romanian",
	"bg":    "Bulgarian",
	"hr":    "Croatian",
	"sk":    "Slovak",
	"he":    "Hebrew",
	"fa":    "Persian",
}

// ResolveLanguage converts an ISO 639-1 code to a full language name.
// If the code is not recognized, it is returned as-is (allowing free-form names).
func ResolveLanguage(code string) string {
	if name, ok := languageNames[code]; ok {
		return name
	}
	return code
}
