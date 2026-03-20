package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var geminiGuideCmd = &cobra.Command{
	Use:   "gemini-guide",
	Short: "Gemini TTS voice steering and prompting guide",
	Long:  "Show detailed guide for Gemini TTS --style prompting, bracket tags, and best practices.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(geminiGuideText)
	},
}

func init() {
	rootCmd.AddCommand(geminiGuideCmd)
}

const geminiGuideText = `
GEMINI TTS VOICE STEERING GUIDE
================================

Gemini TTS voices (bare names like Achernar, Kore, Puck) support voice steering
via the --style flag when using GCP OAuth2 authentication.

  prompt-tools speak "Hello world" --voice Achernar \
    --style "speak warmly and professionally" -o hello.wav


REQUIREMENTS
------------
  - GCP OAuth2 auth: gcloud auth application-default login
  - Gemini voice (bare name, not en-US-Chirp3-HD-*)
  - --style flag sets the "prompt" field in the Cloud TTS API


THE THREE LEVERS OF SPEECH CONTROL
-----------------------------------
For best results, align all three:

  1. Style Prompt (--style) — overall emotional tone and delivery
  2. Text Content — the semantic meaning of the words
  3. Bracket Tags — localized modifications within the text

A scared style prompt works best with text like "I think someone is in the house."
A scared style prompt with "The meeting is at 4 PM." will produce ambiguous results.


STYLE PROMPT EXAMPLES
---------------------
The --style flag accepts natural language instructions:

  --style "Say the following in a curious way"
  --style "Say the following in an elated way"
  --style "You are having a casual conversation with a friend. Say the following in a friendly and amused way."
  --style "Narrate in a calm, professional tone for a documentary."
  --style "You are an AI assistant speaking in a friendly and helpful tone."
  --style "Narrate this in the calm, authoritative tone of a nature documentary narrator."

The more specific your style prompt, the more reliable the result.


BRACKET TAGS
------------
Bracket tags like [sigh] can be embedded directly in the text for localized effects.

  Non-speech sounds (inserted inline):
    [sigh]       Insert a sigh sound
    [laughing]   Insert a laugh
    [uhm]        Insert a hesitation sound

  Style modifiers (affect subsequent speech):
    [sarcasm]        Sarcastic tone on the next phrase
    [robotic]        Robotic-sounding speech
    [shouting]       Increased volume
    [whispering]     Decreased volume
    [extremely fast] Increased speed

  Pacing and pauses:
    [short pause]    Brief pause, like a comma (~250ms)
    [medium pause]   Standard pause, like a sentence break (~500ms)
    [long pause]     Dramatic pause (~1000ms+)

  Vocalized markup (the tag word is spoken, and tone changes):
    [scared]     Word "scared" is spoken; sentence adopts scared tone
    [curious]    Word "curious" is spoken; sentence adopts curious tone
    [bored]      Word "bored" is spoken; sentence adopts bored tone
    Note: because the tag word itself is spoken, this mode may be undesired
    for most use cases.

Examples:
  prompt-tools speak "[extremely fast] Terms and conditions may apply." \
    --voice Achernar --style "Read the following disclaimer" -o disclaimer.wav

  prompt-tools speak "The answer is... [long pause] ...no." \
    --voice Kore --style "Speak dramatically" -o dramatic.wav

  prompt-tools speak "OK, so... tell me about this [uhm] AI thing." \
    --voice Puck --style "Say the following in a curious way" -o curious.wav


BEST PRACTICES
--------------
  - Write specific, detailed prompts — vague prompts produce vague results.
  - Use emotionally rich text — don't rely on prompts and tags alone.
  - Align style, text, and tags — all three should be semantically consistent.
  - Test new tag/prompt combinations before deploying to production.
  - Tag behavior is not always predictable for untested combinations.


LIMITS
------
  - Text field: max 4,000 bytes
  - Style prompt: max 4,000 bytes
  - Combined: max 8,000 bytes
  - Output audio: max ~655 seconds (truncated if exceeded)


SUPPORTED LANGUAGES (GA)
-------------------------
  ar-EG  Arabic (Egypt)           mr-IN  Marathi (India)
  bn-BD  Bangla (Bangladesh)      pl-PL  Polish (Poland)
  de-DE  German (Germany)         pt-BR  Portuguese (Brazil)
  en-IN  English (India)          ro-RO  Romanian (Romania)
  en-US  English (United States)  ru-RU  Russian (Russia)
  es-ES  Spanish (Spain)          ta-IN  Tamil (India)
  fr-FR  French (France)          te-IN  Telugu (India)
  hi-IN  Hindi (India)            th-TH  Thai (Thailand)
  id-ID  Indonesian (Indonesia)   tr-TR  Turkish (Turkey)
  it-IT  Italian (Italy)          uk-UA  Ukrainian (Ukraine)
  ja-JP  Japanese (Japan)         vi-VN  Vietnamese (Vietnam)
  ko-KR  Korean (South Korea)
  nl-NL  Dutch (Netherlands)

  60+ additional languages available in preview, including:
  cmn-CN  Mandarin (China)        en-AU  English (Australia)
  cmn-TW  Mandarin (Taiwan)       en-GB  English (United Kingdom)
  fr-CA   French (Canada)         es-MX  Spanish (Mexico)

Use --language to set the language code for Gemini voices:
  prompt-tools speak "Bonjour le monde" --voice Achernar --language fr-FR -o bonjour.wav


BULK PROCESSING
---------------
Style and Language columns are supported in bulk spreadsheets (columns H and I):

  Filename     | Voice    | Text            | ... | Style              | Language
  welcome.wav  | Achernar | Welcome to...   | ... | speak warmly       | en-US
  bonjour.wav  | Kore     | Bonjour le...   | ... | ton chaleureux     | fr-FR

See: prompt-tools bulk template --output template.xlsx


OFFICIAL DOCUMENTATION
----------------------
  https://cloud.google.com/text-to-speech/docs/gemini-tts

`
