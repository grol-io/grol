// Package mappings some go built in functions to grol functions.
// Same mechanism can be used to map other go functions to grol functions and further extend the language.
package extensions

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fortio.org/log"
	"fortio.org/terminal"
	"github.com/ldemailly/go-scratch/safecast"
	"github.com/rivo/uniseg"
	"grol.io/grol/eval"
	"grol.io/grol/lexer"
	"grol.io/grol/object"
)

var (
	initDone  = false
	errInInit error
	// These are a bit ugly as globals, maybe lambda capture and/or receivers on config instead.
	unrestrictedIOs = false
	emptyOnly       = false
)

const GrolFileExtension = ".gr" // Also the default filename for LoadSaveEmptyOnly.

// Configure restrictions and features.
// Currently about IOs of load and save functions.
type Config struct {
	HasLoad           bool // load() only present if this is true.
	HasSave           bool // save() only present if this is true.
	LoadSaveEmptyOnly bool // Restrict load/save to a single .gr file inside the current directory.
	UnrestrictedIOs   bool // Dangerous when true: can overwrite files, read any readable file etc...
}

// Init initializes the extensions, can be called multiple time safely but should really be called only once
// before using GROL repl/eval. If the passed [Config] pointer is nil, default (safe) values are used.
func Init(c *Config) error {
	if initDone {
		return errInInit
	}
	if c == nil {
		c = &Config{}
	}
	errInInit = initInternal(c)
	initDone = true
	return errInInit
}

func MustCreate(ext object.Extension) {
	err := object.CreateFunction(ext)
	if err != nil {
		panic(err)
	}
}

type OneFloatInOutFunc func(float64) float64

func initInternal(c *Config) error {
	unrestrictedIOs = c.UnrestrictedIOs
	emptyOnly = c.LoadSaveEmptyOnly

	// -- These AddEvalResult should probably be like for discord bot,
	// a separate grol library file embedded in the binary and read/saved in state instead.

	// for printf, we could expose current eval "Out", but instead let's use new variadic support and define
	// printf as print(snprintf(format,..)) that way the memoization of output also works out of the box.
	err := eval.AddEvalResult("printf", "func(format, ..){print(sprintf(format, ..))}")
	if err != nil {
		return err
	}
	err = eval.AddEvalResult("abs", "func(x){if x < 0 {-x} else {x}}")
	if err != nil {
		return err
	}
	err = eval.AddEvalResult("keys", "func(m){if len(m)==0{return []}return [(first(m).key)]+self(rest(m))}")
	if err != nil {
		return err
	}
	err = eval.AddEvalResult("log2", "func(x) {ln(x)/ln(2)}")
	if err != nil {
		return err
	}
	object.AddIdentifier("nil", object.NULL)
	object.AddIdentifier("null", object.NULL)
	object.AddIdentifier("NaN", object.Float{Value: math.NaN()})
	object.AddIdentifier("Inf", object.Float{Value: math.Inf(0)}) // Works for both -Inf and +Inf (thanks to noop unary +).
	object.AddIdentifier("PI", object.Float{Value: math.Pi})
	object.AddIdentifier("E", object.Float{Value: math.E}) // using uppercase so "e" isn't taken/shadowed.

	// --- Real extension section starts here.

	// First one we don't use MustCreate in case it'd error out and we want to return that error.
	err = object.CreateFunction(object.Extension{
		Name:     "pow",
		MinArgs:  2,
		MaxArgs:  2,
		ArgTypes: []object.Type{object.FLOAT, object.FLOAT},
		Callback: object.ShortCallback(pow),
	})
	if err != nil {
		return err
	}
	// Next ones we don't want to keep adding if err != nil ..., so we use MustCreate.
	MustCreate(object.Extension{
		Name:     "sprintf",
		MinArgs:  1,
		MaxArgs:  -1,
		ArgTypes: []object.Type{object.STRING},
		Callback: object.ShortCallback(sprintf),
	})

	oneFloat := object.Extension{
		MinArgs:  1,
		MaxArgs:  1,
		ArgTypes: []object.Type{object.FLOAT},
	}
	for _, function := range []struct {
		fn   OneFloatInOutFunc
		name string
	}{
		{math.Sin, "sin"},
		{math.Cos, "cos"},
		{math.Tan, "tan"},
		{math.Log, "ln"}, // proper name for natural logarithm and also doesn't conflict with logger builtin.
		{math.Sqrt, "sqrt"},
		{math.Exp, "exp"},
		{math.Asin, "asin"},
		{math.Acos, "acos"},
		{math.Atan, "atan"},
		{math.Log10, "log10"},
		{math.Floor, "floor"},
		{math.Ceil, "ceil"},
	} {
		oneFloat.Callback = object.ShortCallback(func(args []object.Object) object.Object {
			// Arg len check already done through MinArgs=MaxArgs=1 and
			// type through ArgTypes: []object.Type{object.FLOAT}.
			return object.Float{Value: function.fn(args[0].(object.Float).Value)}
		})
		oneFloat.Name = function.name
		MustCreate(oneFloat)
	}
	// These are all int-returning functions.
	oneFloat.Name = "round"
	oneFloat.Callback = object.ShortCallback(func(args []object.Object) object.Object {
		return object.Integer{Value: safecast.MustRound[int64](args[0].(object.Float).Value)}
	})
	MustCreate(oneFloat)
	oneFloat.Name = "trunc"
	oneFloat.Callback = object.ShortCallback(func(args []object.Object) object.Object {
		return object.Integer{Value: safecast.MustTruncate[int64](args[0].(object.Float).Value)}
	})
	MustCreate(oneFloat)
	MustCreate(object.Extension{
		Name:     "atan2",
		MinArgs:  2,
		MaxArgs:  2,
		ArgTypes: []object.Type{object.FLOAT, object.FLOAT},
		Callback: object.ShortCallback(func(args []object.Object) object.Object {
			base := args[0].(object.Float).Value
			exp := args[1].(object.Float).Value
			result := math.Atan2(base, exp)
			return object.Float{Value: result}
		}),
	})
	// rand() and rand(n) functions.
	MustCreate(object.Extension{
		Name:     "rand",
		MinArgs:  0,
		MaxArgs:  1,
		ArgTypes: []object.Type{object.INTEGER},
		Callback: func(env any, _ string, args []object.Object) object.Object {
			s := env.(*eval.State)
			if len(args) == 0 {
				return object.Float{Value: rand.Float64()} //nolint:gosec // no need for crypto/rand here.
			}
			n := args[0].(object.Integer).Value
			if n <= 0 {
				return s.NewError("argument to rand() if given must be > 0, >=2 for something useful")
			}
			return object.Integer{Value: rand.Int64N(n)} //nolint:gosec // no need for crypto/rand here.
		},
		DontCache: true,
	})
	createJSONAndEvalFunctions(c)
	createStrFunctions()
	createMisc()
	createTimeFunctions()
	createImageFunctions()
	return nil
}

func createJSONAndEvalFunctions(c *Config) {
	MustCreate(object.Extension{
		Name:     "json_go",
		MinArgs:  1,
		MaxArgs:  2,
		ArgTypes: []object.Type{object.ANY, object.STRING},
		Callback: jsonSerGo,
		Help:     `optional indent e.g json_go(m, "  ")`,
	})
	jsonFn := object.Extension{
		Name:     "json",
		MinArgs:  1,
		MaxArgs:  1,
		ArgTypes: []object.Type{object.ANY},
		Callback: jsonSer,
	}
	MustCreate(jsonFn)
	jsonFn.Name = "type"
	jsonFn.Callback = object.ShortCallback(func(args []object.Object) object.Object {
		obj := args[0]
		if r, ok := obj.(object.Reference); ok {
			return object.String{Value: "&" + r.Name + ".(" + r.Value().Type().String() + ")"}
		}
		return object.String{Value: obj.Type().String()}
	})
	MustCreate(jsonFn)
	jsonFn.Name = "eval"
	jsonFn.Callback = evalFunc
	jsonFn.ArgTypes = []object.Type{object.STRING}
	MustCreate(jsonFn)
	jsonFn.Name = "unjson"
	jsonFn.Callback = evalFunc // unjson at the moment is just (like) eval hoping that json is map/array/...
	MustCreate(jsonFn)

	loadSaveFn := object.Extension{
		MinArgs:  0, // empty only case - ie ".gr" save file.
		MaxArgs:  1,
		ArgTypes: []object.Type{object.STRING},
		Help:     "filename (.gr)",
	}
	if c.HasSave {
		loadSaveFn.Name = "save"
		loadSaveFn.Callback = saveFunc // save to file.
		MustCreate(loadSaveFn)
	}
	if c.HasLoad {
		loadSaveFn.Name = "load"
		loadSaveFn.Callback = loadFunc // eval a file.
		MustCreate(loadSaveFn)
	}
}

func createStrFunctions() {
	strFn := object.Extension{
		MinArgs:  1,
		MaxArgs:  1,
		ArgTypes: []object.Type{object.STRING},
	}
	strFn.Name = "runes" // like explode.gr explodeRunes but go side and not recursive.
	strFn.Callback = func(_ any, _ string, args []object.Object) object.Object {
		inp := args[0].(object.String).Value
		gorunes := []rune(inp)
		l := len(gorunes)
		object.MustBeOk(l)
		runes := make([]object.Object, l)
		for i, r := range gorunes {
			runes[i] = object.String{Value: string(r)}
		}
		return object.NewArray(runes)
	}
	MustCreate(strFn)
	strFn.Name = "rune_len"
	strFn.Callback = func(_ any, _ string, args []object.Object) object.Object {
		return object.Integer{Value: int64(utf8.RuneCountInString(args[0].(object.String).Value))}
	}
	MustCreate(strFn)
	strFn.Name = "width"
	strFn.Callback = func(_ any, _ string, args []object.Object) object.Object {
		return object.Integer{Value: int64(uniseg.StringWidth((args[0].(object.String).Value)))}
	}
	MustCreate(strFn)
	strFn.Name = "split"
	strFn.Help = "optional separator"
	strFn.MinArgs = 1
	strFn.MaxArgs = 2
	strFn.ArgTypes = []object.Type{object.STRING, object.STRING}
	strFn.Callback = func(_ any, _ string, args []object.Object) object.Object {
		inp := args[0].(object.String).Value
		sep := ""
		if len(args) == 2 {
			sep = args[1].(object.String).Value
		}
		parts := strings.Split(inp, sep)
		l := len(parts)
		object.MustBeOk(l)
		strs := make([]object.Object, l)
		for i, p := range parts {
			strs[i] = object.String{Value: p}
		}
		return object.NewArray(strs)
	}
	MustCreate(strFn)
	strFn.Name = "join"
	strFn.Help = "joins an array of string with the optional separator"
	strFn.ArgTypes = []object.Type{object.ARRAY, object.STRING}
	strFn.Callback = func(_ any, _ string, args []object.Object) object.Object {
		arr := object.Elements(args[0])
		sep := ""
		if len(args) == 2 {
			sep = args[1].(object.String).Value
		}
		strs := make([]string, len(arr))
		totalLen := 0
		sepLen := len(sep)
		for i, a := range arr {
			if a.Type() != object.STRING {
				strs[i] = a.Inspect()
			} else {
				strs[i] = a.(object.String).Value
			}
			totalLen += len(strs[i]) + sepLen
		}
		object.MustBeOk(totalLen / object.ObjectSize) // off by sepLen but that's ok.
		return object.String{Value: strings.Join(strs, sep)}
	}
	MustCreate(strFn)
}

func createMisc() {
	minMaxFn := object.Extension{
		MinArgs:  1,
		MaxArgs:  -1,
		ArgTypes: []object.Type{object.ANY},
	}
	minMaxFn.Name = "min"
	minMaxFn.Callback = func(_ any, _ string, args []object.Object) object.Object {
		if len(args) == 1 {
			return args[0]
		}
		minV := args[0]
		for _, a := range args[1:] {
			if object.Cmp(a, minV) < 0 {
				minV = a
			}
		}
		return minV
	}
	MustCreate(minMaxFn)
	minMaxFn.Name = "max"
	minMaxFn.Callback = func(_ any, _ string, args []object.Object) object.Object {
		if len(args) == 1 {
			return args[0]
		}
		maxV := args[0]
		for _, a := range args[1:] {
			if object.Cmp(a, maxV) > 0 {
				maxV = a
			}
		}
		return maxV
	}
	MustCreate(minMaxFn)

	intFn := object.Extension{
		Name:     "int",
		MinArgs:  1,
		MaxArgs:  1,
		ArgTypes: []object.Type{object.ANY},
		Callback: func(st any, _ string, args []object.Object) object.Object {
			s := st.(*eval.State)
			o := args[0]
			switch o.Type() {
			case object.INTEGER:
				return o
			case object.NIL:
				return object.Integer{Value: 0}
			case object.BOOLEAN:
				if o.(object.Boolean).Value {
					return object.Integer{Value: 1}
				}
				return object.Integer{Value: 0}
			case object.FLOAT:
				return object.Integer{Value: int64(o.(object.Float).Value)}
			case object.STRING:
				i, serr := strconv.ParseInt(o.(object.String).Value, 0, 64)
				if serr != nil {
					return s.Error(serr)
				}
				return object.Integer{Value: i}
			default:
				return s.Errorf("cannot convert %s to int", o.Type())
			}
		},
	}
	MustCreate(intFn)
	intFn.Name = "base64"
	intFn.Callback = func(st any, _ string, args []object.Object) object.Object {
		s := st.(*eval.State)
		o := args[0]
		var data []byte
		switch o.Type() {
		case object.REFERENCE:
			ref := o.(object.Reference)
			if ref.Value().Type() != object.STRING {
				return s.Errorf("cannot convert ref to %s to base64", ref.Value().Type())
			}
			data = []byte(ref.Value().(object.String).Value)
		case object.STRING:
			data = []byte(o.(object.String).Value)
		default:
			return s.Errorf("cannot convert %s to base64", o.Type())
		}
		encoded := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
		base64.StdEncoding.Encode(encoded, data)
		return object.String{Value: string(encoded)}
	}
	MustCreate(intFn)
}

func createTimeFunctions() {
	MustCreate(object.Extension{
		Name:    "time",
		MinArgs: 0,
		MaxArgs: 0,
		Help:    "Date/time in seconds since epoch",
		Callback: object.ShortCallback(func(_ []object.Object) object.Object {
			return object.Float{Value: float64(time.Now().UnixMicro()) / 1e6}
		}),
		DontCache: true,
	})
	MustCreate(object.Extension{
		Name:     "sleep",
		MinArgs:  1,
		MaxArgs:  1,
		ArgTypes: []object.Type{object.FLOAT},
		Help:     "in seconds",
		Callback: func(st any, _ string, args []object.Object) object.Object {
			s := st.(*eval.State)
			durSec := args[0].(object.Float).Value
			if durSec < 0 {
				return s.NewError("negative sleep duration")
			}
			durDur := time.Duration(durSec * 1e9)
			log.Infof("Sleeping for %v", durDur)
			return s.Error(terminal.SleepWithContext(s.Context, durDur))
		},
	})
	MustCreate(object.Extension{
		Name:     "time_info",
		MinArgs:  1,
		MaxArgs:  2,
		ArgTypes: []object.Type{object.FLOAT, object.STRING},
		Help:     "Float as returned by time() and time_parse() in seconds since epoch, and optional TimeZone/location",
		Callback: func(st any, _ string, args []object.Object) object.Object {
			s := st.(*eval.State)
			timeUsec := math.Round(args[0].(object.Float).Value * 1e6)
			// parse_time without a TZ will be in UTC, so to echo it back the same we also default to UTC.
			// caller can pass "Local" to get the local time.
			t := time.UnixMicro(int64(timeUsec)).UTC()
			if len(args) == 2 {
				timeZone := args[1].(object.String).Value
				if strings.EqualFold("local", timeZone) {
					timeZone = "Local"
				}
				location, err := time.LoadLocation(timeZone)
				if err != nil {
					return s.Error(err)
				}
				t = t.In(location)
			}
			usec := int64(timeUsec) % 1e6
			formattedTime := t.Format("2006-01-02 15:04:05.999999")
			log.Debugf("Time is for %v", t)
			m := &object.BigMap{}
			m.Set(object.String{Value: "str"}, object.String{Value: formattedTime})
			m.Set(object.String{Value: "year"}, object.Integer{Value: int64(t.Year())})
			m.Set(object.String{Value: "month"}, object.Integer{Value: int64(t.Month())})
			m.Set(object.String{Value: "day"}, object.Integer{Value: int64(t.Day())})
			m.Set(object.String{Value: "hour"}, object.Integer{Value: int64(t.Hour())})
			m.Set(object.String{Value: "minute"}, object.Integer{Value: int64(t.Minute())})
			m.Set(object.String{Value: "second"}, object.Integer{Value: int64(t.Second())})
			m.Set(object.String{Value: "weekday"}, object.Integer{Value: int64(t.Weekday())})
			name, offset := t.Zone()
			m.Set(object.String{Value: "tz"}, object.String{Value: name})
			m.Set(object.String{Value: "offset"}, object.Integer{Value: int64(offset)})
			m.Set(object.String{Value: "usec"}, object.Integer{Value: usec})
			return m
		},
	})
	MustCreate(object.Extension{
		Name:     "time_parse",
		MinArgs:  1,
		MaxArgs:  2,
		ArgTypes: []object.Type{object.STRING, object.STRING},
		Help:     "Parse a time string with optional format, returns seconds since epoch",
		Callback: func(st any, _ string, args []object.Object) object.Object {
			s := st.(*eval.State)
			inp := args[0].(object.String).Value
			if len(args) == 1 {
				t, err := TryParseTime(inp)
				if err != nil {
					return s.Error(err)
				}
				return object.Float{Value: float64(t.UnixMicro()) / 1e6}
			}
			format := args[1].(object.String).Value
			t, err := time.Parse(format, inp)
			if err != nil {
				return s.Error(err)
			}
			return object.Float{Value: float64(t.UnixMicro()) / 1e6}
		},
	})
}

// --- implementation of the functions that aren't inlined in lambdas above.

var parseFormats = []string{
	time.DateTime, //   = "2006-01-02 15:04:05" // first as that's what time_info().str returns (with usec).
	time.RFC3339,
	time.ANSIC,    //   = "Mon Jan _2 15:04:05 2006"
	time.UnixDate, //   = "Mon Jan _2 15:04:05 MST 2006"
	time.RFC822,   //   = "02 Jan 06 15:04 MST"
	time.RFC822Z,  //   = "02 Jan 06 15:04 -0700" // RFC822 with numeric zone
	time.RFC850,   //   = "Monday, 02-Jan-06 15:04:05 MST"
	time.RFC1123,  //   = "Mon, 02 Jan 2006 15:04:05 MST"
	time.RFC1123Z, //   = "Mon, 02 Jan 2006 15:04:05 -0700" // RFC1123 with numeric zone
	time.RFC3339,  //   = "2006-01-02T15:04:05Z07:00"
	time.Kitchen,  //   = "3:04PM"
	time.Stamp,    //   = "Jan _2 15:04:05"
	time.DateOnly, //   = "2006-01-02"
	time.TimeOnly, //   = "15:04:05"
	"_2 Jan 2006",
	"_2/1/2006", // try EU (ie sensible) style first.
	"1/_2/2006",
	"_2-Jan-2006",
	"02/01/06",
	"01/02/06",
}

func TryParseTime(input string) (time.Time, error) {
	var t time.Time
	var err error
	for i, format := range parseFormats { // maybe consider grouping formats by length
		t, err = time.Parse(format, input)
		if err == nil {
			log.Infof("Parsed %q with format#%d: %q to %v", input, i+1, format, t)
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %v", input)
}

func pow(args []object.Object) object.Object {
	// Arg len check already done through MinArgs and MaxArgs
	// and so is type check through ArgTypes.
	base := args[0].(object.Float).Value
	exp := args[1].(object.Float).Value
	result := math.Pow(base, exp)
	return object.Float{Value: result}
}

func sprintf(args []object.Object) object.Object {
	res := fmt.Sprintf(args[0].(object.String).Value, object.Unwrap(args[1:], false)...)
	return object.String{Value: res}
}

func jsonSer(env any, _ string, args []object.Object) object.Object {
	s := env.(*eval.State)
	w := strings.Builder{}
	err := args[0].JSON(&w)
	if err != nil {
		return s.Error(err)
	}
	return object.String{Value: w.String()}
}

func jsonSerGo(env any, _ string, args []object.Object) object.Object {
	s := env.(*eval.State)
	v := args[0].Unwrap(true)
	var err error
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	if len(args) == 2 {
		encoder.SetIndent("", args[1].(object.String).Value)
	}
	// Disable HTML escaping
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(v)
	if err != nil {
		return s.Error(err)
	}
	return object.String{Value: buf.String()}
}

func evalFunc(env any, name string, args []object.Object) object.Object {
	str := args[0].(object.String).Value
	s := env.(*eval.State)
	res, err := eval.EvalString(s, str, name == "unjson" /* empty env */)
	if err != nil {
		return s.Error(err)
	}
	return res
}

// Normalizes to alphanum.gr.
func sanitizeFileName(args []object.Object) (string, error) {
	if len(args) == 0 {
		return GrolFileExtension, nil
	}
	file := args[0].(object.String).Value
	if emptyOnly && file != "" {
		return "", fmt.Errorf("empty only mode, filename must be empty or no arguments, got: %q", file)
	}
	if unrestrictedIOs {
		log.Infof("Unrestricted IOs, not sanitizing filename: %s", file)
		return file, nil
	}
	// only alhpanumeric and _ allowed. no dots, slashes, etc.
	f := strings.TrimSuffix(file, GrolFileExtension)
	for _, r := range []byte(f) {
		if !lexer.IsAlphaNum(r) {
			return "", fmt.Errorf("invalid character in filename %q: %c", file, r)
		}
	}
	return f + GrolFileExtension, nil
}

func saveFunc(env any, _ string, args []object.Object) object.Object {
	s := env.(*eval.State)
	file, err := sanitizeFileName(args)
	if err != nil {
		return s.Error(err)
	}
	f, err := os.Create(file)
	if err != nil {
		return s.Error(err)
	}
	defer f.Close()
	// Write to file.
	n, err := s.SaveGlobals(f)
	if err != nil {
		return s.Error(err)
	}
	log.Infof("Saved %d ids/fns to: %s", n, file)
	return object.MakeQuad(
		object.String{Value: "entries"}, object.Integer{Value: int64(n)},
		object.String{Value: "filename"}, object.String{Value: file})
}

func loadFunc(env any, _ string, args []object.Object) object.Object {
	file, err := sanitizeFileName(args)
	s := env.(*eval.State)
	if err != nil {
		return s.Error(err)
	}
	f, err := os.Open(file)
	if err != nil {
		return s.Error(err)
	}
	defer f.Close()
	all, err := io.ReadAll(f)
	if err != nil {
		return s.Error(err)
	}
	// Eval the content.
	res, err := eval.EvalString(env, string(all), false)
	if err != nil {
		return s.Error(err)
	}
	log.Infof("Read/evaluated: %s", file)
	return res
}
