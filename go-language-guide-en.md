# The Complete Go Language Reference
### A Principal-Level Guide to Every Go Language Feature — What It Is, How It Works, and When to Use It

> This document covers the Go language itself (not the standard library at large), from first principles to the subtle internals that separate "I can write Go" from "I understand Go." Code samples are runnable Go; explanations focus on *mechanism* and *intent*.

---

## Table of Contents

1. [Philosophy & Design Goals](#1-philosophy--design-goals)
2. [Source Files, Packages & Program Structure](#2-source-files-packages--program-structure)
3. [Variables, Constants & `iota`](#3-variables-constants--iota)
4. [Basic Types & Zero Values](#4-basic-types--zero-values)
5. [Pointers](#5-pointers)
6. [Arrays vs. Slices — Internals](#6-arrays-vs-slices--internals)
7. [Maps](#7-maps)
8. [Strings, Runes, Bytes & UTF-8](#8-strings-runes-bytes--utf-8)
9. [Control Flow](#9-control-flow)
10. [Functions](#10-functions)
11. [Closures](#11-closures)
12. [Structs](#12-structs)
13. [Struct Embedding](#13-struct-embedding)
14. [Struct Tags & Reflection-Driven Metadata](#14-struct-tags--reflection-driven-metadata)
15. [Methods & Method Sets](#15-methods--method-sets)
16. [Interfaces](#16-interfaces)
17. [Generics (Type Parameters)](#17-generics-type-parameters)
18. [Error Handling](#18-error-handling)
19. [Defer, Panic & Recover](#19-defer-panic--recover)
20. [Goroutines & the Go Scheduler](#20-goroutines--the-go-scheduler)
21. [Channels & `select`](#21-channels--select)
22. [The `sync` Package](#22-the-sync-package)
23. [`sync/atomic`](#23-syncatomic)
24. [`context` Package](#24-context-package)
25. [Packages, Modules & Visibility](#25-packages-modules--visibility)
26. [Internal Packages](#26-internal-packages)
27. [`go:embed` — Embedded Filesystems](#27-goembed--embedded-filesystems)
28. [`go generate` — Code Generation](#28-go-generate--code-generation)
29. [Build Constraints (`//go:build`)](#29-build-constraints-gobuild)
30. [Reflection (`reflect`)](#30-reflection-reflect)
31. [Range-over-Func Iterators (Go 1.23+)](#31-range-over-func-iterators-go-123)
32. [Compiler Directives](#32-compiler-directives)
33. [Escape Analysis & the Go Memory Model](#33-escape-analysis--the-go-memory-model)
34. [Testing, Benchmarking & Fuzzing](#34-testing-benchmarking--fuzzing)
35. [The Blank Identifier `_`](#35-the-blank-identifier-_)
36. [Labels, `goto`, `break`, `continue`](#36-labels-goto-break-continue)
37. [`init()` Functions](#37-init-functions)
38. [Idioms & Best Practices Checklist](#38-idioms--best-practices-checklist)

---

## 1. Philosophy & Design Goals

Go was designed at Google (Rob Pike, Ken Thompson, Robert Griesemer, first released 2009) to solve an *engineering* problem, not a language-theory problem: large teams, large codebases, slow C++ build times, and the operational cost of concurrency in production systems.

Core design pillars:

- **Simplicity over expressiveness.** Go deliberately omits features (classical inheritance, operator overloading, exceptions, macros) that increase cognitive load in large codebases with many authors.
- **Fast compilation.** The dependency model, lack of circular imports, and explicit imports exist specifically to make builds parallelizable and fast.
- **Concurrency as a first-class citizen.** Goroutines and channels are language features, not library add-ons (unlike threads in most languages).
- **Explicitness over magic.** Errors are values you must handle explicitly; there's no hidden control flow via exceptions.
- **Composition over inheritance.** Interfaces and embedding replace class hierarchies.
- **One way to format code.** `gofmt` removes bikeshedding entirely — this is a *language-level cultural feature*, not just tooling.

Understanding *why* these choices were made explains almost every "weird" thing about Go — no generics for a decade (added 1.18, 2022), no exceptions, unused imports/variables being compile errors, etc.

---

## 2. Source Files, Packages & Program Structure

Every `.go` file belongs to exactly one package, declared at the top:

```go
package main // executable entry point package

import (
    "fmt"
    "os"
)

func main() {
    fmt.Println("Hello, Go")
    os.Exit(0)
}
```

- `package main` + a `func main()` produces an executable.
- Any other package name produces a library package, importable by others.
- All files in the same directory must declare the same package name (with the narrow exception of `_test` packages, e.g. `foo_test`, used for black-box tests).
- Package names are lowercase, short, no underscores (`net/http`, not `net/HTTP` or `net_http`).
- **Import paths are not the package name** — the import path is a location (often a repo path like `github.com/user/repo`), while the *identifier* used in code is the `package` clause inside the files (usually the last path element, but not required to match).

```go
import (
    "fmt"
    myjson "encoding/json" // aliased import
    _ "image/png"          // blank import: side-effect only (registers a decoder)
)
```

A blank import (`_ "image/png"`) runs the package's `init()` functions for their side effects (e.g., registering codecs, drivers) without using any of its exported identifiers directly.

---

## 3. Variables, Constants & `iota`

### Variable declarations

```go
var x int              // zero-valued (0)
var y int = 10
var z = 10              // type inferred
a := 10                 // short declaration; only inside functions
var (
    b int
    c string
    d bool
)
```

`:=` cannot be used at package scope, and requires at least one new variable on the left-hand side on redeclaration (`a, err := f()` then later `a, err = g()` or `a, err := h()` if `a` is reused but at least one identifier — here none — must be new; Go allows this specific redeclare-in-multi-assign pattern only when at least one variable on the left is new).

### Constants

Constants are computed at compile time and can be untyped, which lets them adapt to context:

```go
const Pi = 3.14159       // untyped constant, usable as float32, float64, etc.
const MaxUsers int = 100 // typed constant

const (
    KB = 1 << (10 * (iota + 1)) // 1024
    MB                          // 1048576
    GB                          // 1073741824
)
```

### `iota`

`iota` is a compile-time counter reset to `0` in every `const (...)` block and incremented by one per `ConstSpec` line (not per identifier used). It's Go's idiomatic replacement for enums:

```go
type Weekday int

const (
    Sunday Weekday = iota // 0
    Monday                // 1
    Tuesday               // 2
    Wednesday             // 3
    Thursday              // 4
    Friday                // 5
    Saturday              // 6
)

func (d Weekday) String() string {
    names := [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
    if d < 0 || int(d) >= len(names) {
        return "Unknown"
    }
    return names[d]
}
```

Skipping values with `_`:

```go
const (
    _  = iota             // skip 0
    KB = 1 << (10 * iota) // 1<<10
    MB                    // 1<<20
    GB                    // 1<<30
)
```

**When to use:** any closed set of related integer constants (states, flags, levels, days). Prefer `iota`-based enums plus a `String()` method (satisfying `fmt.Stringer`) over raw string constants for type safety and cheap comparisons.

---

## 4. Basic Types & Zero Values

| Category | Types | Zero value |
|---|---|---|
| Boolean | `bool` | `false` |
| Numeric (signed) | `int, int8, int16, int32, int64` | `0` |
| Numeric (unsigned) | `uint, uint8, uint16, uint32, uint64, uintptr` | `0` |
| Floating point | `float32, float64` | `0.0` |
| Complex | `complex64, complex128` | `0+0i` |
| String | `string` | `""` |
| Alias | `byte` (=`uint8`), `rune` (=`int32`) | `0` |

Key facts:

- **`int`/`uint` size is platform-dependent** (32 or 64 bits, virtually always 64-bit on modern targets) — don't assume a fixed width; use `int32`/`int64` explicitly when wire format or overflow behavior matters (e.g., serialization, hashing).
- **No implicit numeric conversions.** `var f float64 = 3; var i int = f` is a compile error — you must write `int(f)`. This eliminates an entire class of silent-precision-loss bugs common in C/Java.
- **Every declared variable has a zero value** — there is no "uninitialized" memory in Go at the language level; this is a deliberate safety guarantee.
- **Strings are immutable** byte sequences; `[]byte(s)` and `string(b)` perform O(n) copies.

```go
var s string           // ""
var arr [3]int         // [0 0 0]
var sl []int           // nil, len=0, cap=0 — usable, not the same as empty slice literal
var m map[string]int   // nil — reads return zero value, writes panic
var p *int             // nil
var ifc interface{}    // nil
var ch chan int        // nil — sends/receives block forever
```

---

## 5. Pointers

Go has pointers but no pointer arithmetic (unlike C). A pointer holds the address of a value.

```go
x := 42
p := &x        // p is *int, holds address of x
*p = 100       // dereference and assign; x is now 100
fmt.Println(x) // 100

var np *int    // nil pointer
// *np = 1     // panic: nil pointer dereference
```

- `new(T)` allocates zeroed memory for a `T` and returns `*T`. It's rarely used directly since composite literals (`&T{}`) are more idiomatic for structs.
- Passing a pointer avoids copying large structs and allows a function to mutate the caller's value.
- Go **automatically dereferences** pointers for field access and method calls: `p.Field` is shorthand for `(*p).Field`.

```go
type Counter struct{ n int }
func (c *Counter) Inc() { c.n++ } // pointer receiver — mutates the original

c := Counter{}
c.Inc() // Go automatically takes &c here because Inc has a pointer receiver
```

**When to use pointers:** when a function needs to mutate the argument, when copying the value is expensive (large structs), or to represent optionality (`*bool` to distinguish "false" from "not set"). Avoid pointers for small immutable values (`int`, `string`) — passing by value is often faster and clearer.

---

## 6. Arrays vs. Slices — Internals

### Arrays

Fixed-length, value type. `[4]int` and `[5]int` are *different types* entirely, and arrays are copied on assignment/passing.

```go
var a [4]int
b := a         // full copy of the array
b[0] = 100     // does not affect a
```

### Slices

A slice is a **header** — a small struct with three fields — pointing at an underlying array:

```go
type sliceHeader struct {
    ptr *T
    len int
    cap int
}
```

```go
s := make([]int, 3, 10) // len=3, cap=10
s = append(s, 4)        // len=4, still within cap: reuses same backing array
```

**Growth semantics:** when `append` exceeds `cap`, Go allocates a new, larger backing array (historically ~2x for small slices, tapering to ~1.25x for large slices — the exact factor is a runtime implementation detail, not a spec guarantee) and copies all elements over. Because of this:

```go
a := make([]int, 0, 4)
b := append(a, 1)
c := append(a, 2)
// b and c may or may not share a backing array with a, depending on capacity —
// writing through one can silently corrupt the other. This is one of Go's most
// common footguns.
```

**Slicing shares memory:**

```go
original := []int{1, 2, 3, 4, 5}
sub := original[1:3]  // [2 3], shares backing array with original
sub[0] = 99            // original is now [1 99 3 4 5]
```

Use the **three-index slice expression** `s[low:high:max]` to control capacity explicitly and force a full copy on next append, preventing accidental aliasing:

```go
sub := original[1:3:3] // cap(sub) == 2, next append will allocate fresh memory
```

**`copy()`** performs an explicit, safe deep copy of overlapping elements:

```go
dst := make([]int, len(src))
n := copy(dst, src) // n = number of elements copied
```

**When to use arrays vs. slices:** arrays almost never appear in idiomatic Go APIs (fixed-size buffers, small crypto keys, or as backing storage passed by pointer are the main exceptions). Slices are the default sequence type.

---

## 7. Maps

Maps are hash tables, reference types (like slices, they're headers pointing to underlying data).

```go
m := make(map[string]int)
m["a"] = 1

v, ok := m["missing"] // v=0 (zero value), ok=false — the "comma ok" idiom
delete(m, "a")        // safe even if key absent

var nilMap map[string]int
_ = nilMap["x"]        // fine, returns zero value
// nilMap["x"] = 1      // panics: assignment to entry in nil map
```

- **Iteration order is intentionally randomized** by the runtime specifically to prevent developers from relying on it (this was a deliberate design choice added after Go 1 to stop accidental order-dependence).
- Map keys must be **comparable** types (no slices, maps, or funcs as keys — but structs, arrays, and pointers are fine).
- Maps are **not safe for concurrent read/write** — use `sync.Map` or a `sync.RWMutex`-guarded map for concurrent access (see §22).

```go
type Point struct{ X, Y int }
seen := map[Point]bool{}
seen[Point{1, 2}] = true // struct as key — works because Point is comparable
```

---

## 8. Strings, Runes, Bytes & UTF-8

Go strings are **immutable, read-only slices of bytes**, conventionally holding UTF-8 encoded text. There is no dedicated "character" type — you choose between `byte` (raw octet) and `rune` (a Unicode code point, alias for `int32`).

```go
s := "héllo"
fmt.Println(len(s))       // 6 — byte length, NOT character count (é is 2 bytes in UTF-8)

for i, r := range s {
    fmt.Println(i, r) // ranging over a string decodes UTF-8, yields (byteIndex, rune)
}

runes := []rune(s)   // decode into a rune slice
fmt.Println(len(runes)) // 5 — actual character count

b := []byte(s)       // raw byte view
```

- Concatenating strings in a loop (`s += x`) is O(n²) due to immutability; use `strings.Builder` for efficient accumulation:

```go
var sb strings.Builder
for _, w := range words {
    sb.WriteString(w)
    sb.WriteByte(' ')
}
result := sb.String()
```

- Byte-string conversions (`[]byte(s)`, `string(b)`) copy data — except when the compiler can prove the conversion is used only transiently (e.g., `m[string(b)]` map lookups are specially optimized to avoid the copy).

---

## 9. Control Flow

Go has no `while` — `for` is the only loop construct, with four forms:

```go
for i := 0; i < 10; i++ { }     // classic C-style
for cond { }                      // while-style
for { }                            // infinite loop
for i, v := range collection { }  // range loop (works on arrays, slices, strings, maps, channels, ints since 1.22, funcs since 1.23)
```

**`for range` over an integer** (Go 1.22+):

```go
for i := range 5 { // i = 0,1,2,3,4
    fmt.Println(i)
}
```

**Loop variable semantics changed in Go 1.22:** prior to 1.22, the loop variable was reused across iterations (a classic goroutine-capture bug); since 1.22, each iteration gets a fresh variable:

```go
// Pre-1.22, this printed the SAME final value N times.
// Since 1.22, this correctly prints 0..N-1 because each iteration
// has its own `i`.
for i := 0; i < 3; i++ {
    go func() { fmt.Println(i) }()
}
```

### `switch`

No fallthrough by default (opposite of C); use explicit `fallthrough`:

```go
switch x := compute(); {   // switch with no tag, condition per case
case x < 0:
    fmt.Println("negative")
case x == 0:
    fmt.Println("zero")
    fallthrough
default:
    fmt.Println("non-negative")
}
```

**Type switch:**

```go
func describe(i interface{}) {
    switch v := i.(type) {
    case int:
        fmt.Println("int:", v)
    case string:
        fmt.Println("string:", v)
    case nil:
        fmt.Println("nil value")
    default:
        fmt.Printf("unknown type %T\n", v)
    }
}
```

### `if` with init statement

```go
if err := doSomething(); err != nil {
    return err
}
// err is scoped only to the if/else block
```

---

## 10. Functions

Functions are first-class values with multiple return values, named returns, and variadic parameters.

```go
func divmod(a, b int) (int, int) {
    return a / b, a % b
}

// Named returns — declared in the signature, implicitly returned by bare `return`
func split(sum int) (x, y int) {
    x = sum * 4 / 9
    y = sum - x
    return // "naked" return
}

func sum(nums ...int) int { // variadic
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}
sum(1, 2, 3)
sum(sliceOfInts...) // spread a slice into variadic args
```

Functions as values / parameters (higher-order functions):

```go
func apply(fn func(int) int, x int) int { return fn(x) }
squared := apply(func(x int) int { return x * x }, 5)
```

**Multiple return values idiomatically encode "value + error":**

```go
func parse(s string) (int, error) {
    n, err := strconv.Atoi(s)
    if err != nil {
        return 0, fmt.Errorf("parse %q: %w", s, err)
    }
    return n, nil
}
```

---

## 11. Closures

A closure is a function value that references variables from outside its body; those variables are captured **by reference**, not by value.

```go
func counter() func() int {
    n := 0
    return func() int {
        n++
        return n
    }
}

c := counter()
fmt.Println(c()) // 1
fmt.Println(c()) // 2 — n persists between calls, uniquely per closure instance
```

Common uses: middleware/decorator patterns, generating stateful callbacks, `defer`-based cleanup with captured context, and functional options (see below).

**Functional options pattern** (idiomatic Go API design for optional configuration):

```go
type ServerOption func(*Server)

func WithTimeout(d time.Duration) ServerOption {
    return func(s *Server) { s.timeout = d }
}
func WithMaxConns(n int) ServerOption {
    return func(s *Server) { s.maxConns = n }
}

func NewServer(opts ...ServerOption) *Server {
    s := &Server{timeout: 30 * time.Second, maxConns: 100} // defaults
    for _, opt := range opts {
        opt(s)
    }
    return s
}

srv := NewServer(WithTimeout(5*time.Second), WithMaxConns(10))
```

---

## 12. Structs

Structs are typed collections of named fields — Go's only real composite "record" type.

```go
type User struct {
    ID       int
    Name     string
    Email    string
    IsActive bool
}

u := User{ID: 1, Name: "Alice"}       // keyed literal (preferred — order-independent)
u2 := User{2, "Bob", "b@x.com", true} // positional literal — brittle, avoid in public APIs

u3 := new(User) // *User, zero-valued
u4 := &User{}   // *User, zero-valued — more idiomatic than new()
```

Structs are compared with `==` if all fields are comparable:

```go
a := User{ID: 1}
b := User{ID: 1}
fmt.Println(a == b) // true, field-wise comparison
```

Anonymous structs are useful for throwaway groupings (test tables, one-off JSON shapes):

```go
data := struct {
    Name string
    Age  int
}{"Alice", 30}
```

---

## 13. Struct Embedding

Go achieves composition (not inheritance) via embedding — including an unnamed field of another type promotes its fields and methods to the outer type.

```go
type Base struct {
    ID        int
    CreatedAt time.Time
}
func (b Base) Describe() string { return fmt.Sprintf("Base#%d", b.ID) }

type User struct {
    Base          // embedded — no field name, just the type
    Name string
}

u := User{Base: Base{ID: 1}, Name: "Alice"}
fmt.Println(u.ID)          // promoted field access — same as u.Base.ID
fmt.Println(u.Describe())  // promoted method — same as u.Base.Describe()
```

Key rules:

- This is **not** inheritance: `User` does not satisfy an interface requiring `Base` as a type; there's no polymorphic dispatch upward. Method calls promoted from `Base` still receive `Base` as the receiver, not `User` (no virtual dispatch — a `Describe` method on `Base` cannot see `User`'s fields).
- If both the outer struct and the embedded type have a field/method with the same name, the outer one **shadows** the embedded one (shallowest-wins, and it's a compile error only if the ambiguity happens at the *same* depth from two different embedded types).
- Embedding interfaces is equally common and is how many stdlib types compose behavior (`io.ReadWriteCloser` embeds `Reader`, `Writer`, `Closer`).

```go
type ReadWriter interface {
    io.Reader
    io.Writer
}
```

**When to use:** prefer embedding over "has-a" wrapper boilerplate when you genuinely want to expose the embedded type's full API on the outer type (e.g., wrapping `sync.Mutex`, extending a base HTTP handler). Don't embed purely to save typing `.Base` — embedding is part of your public API surface and its promoted methods become part of your contract.

---

## 14. Struct Tags & Reflection-Driven Metadata

A struct tag is a raw string literal attached to a field, parsed at runtime via `reflect`. The language itself does nothing with tags — they're inert metadata that libraries (encoding/json, encoding/xml, validators, ORMs, gRPC) interpret by convention.

```go
type User struct {
    ID    int    `json:"id" db:"user_id"`
    Name  string `json:"name,omitempty" validate:"required,min=2"`
    Email string `json:"-"`               // "-" means: never (de)serialize this field
    Age   int    `json:"age,string"`       // "string" option: encode number as a JSON string
}
```

Tag syntax convention: space-separated `key:"value"` pairs, where `value` itself often has a comma-separated primary value plus options (`name,omitempty`).

Reading tags manually via reflection:

```go
t := reflect.TypeOf(User{})
field, _ := t.FieldByName("Name")
tag := field.Tag.Get("json")        // "name,omitempty"
validateTag := field.Tag.Get("validate") // "required,min=2"
```

**When to use:** any time a struct needs to be serialized/deserialized to an external format (JSON, XML, protobuf via struct-tag-based libraries, database rows via `sqlx`/`gorm`), or validated declaratively. Struct tags are Go's answer to annotations/decorators in other languages — deliberately data-only (no executable code) to preserve Go's "no magic" philosophy; all the actual behavior lives in ordinary, readable Go code in the library that parses the tag.

---

## 15. Methods & Method Sets

A method is a function with a receiver argument, associated with a named type (which must be defined in the same package as the method).

```go
type Rectangle struct{ W, H float64 }

func (r Rectangle) Area() float64 { return r.W * r.H }        // value receiver
func (r *Rectangle) Scale(f float64) { r.W *= f; r.H *= f }   // pointer receiver
```

**Value receiver:** the method operates on a *copy* of the receiver — mutations don't affect the original.
**Pointer receiver:** the method operates on the original via its address — required for mutation, and cheaper for large structs (avoids copying).

### Method sets and interface satisfaction

This is one of the most consequential rules in Go:

- The method set of type `T` consists of all methods declared with receiver `T`.
- The method set of type `*T` consists of all methods declared with receiver `T` *or* `*T`.

Consequence: **a value of type `T` (not `*T`) does NOT satisfy an interface if any required method has a pointer receiver.**

```go
type Shape interface { Scale(float64) }

var s Shape = Rectangle{}   // COMPILE ERROR: Rectangle does not implement Shape (Scale has pointer receiver)
var s Shape = &Rectangle{}  // OK
```

**Rule of thumb:** if *any* method on a type needs a pointer receiver (for mutation or to avoid copying a large struct), make *all* methods on that type use pointer receivers, for consistency and to avoid the trap above.

---

## 16. Interfaces

Go interfaces are **implicitly satisfied** — there's no `implements` keyword. Any type that has the required methods automatically satisfies the interface. This is *structural typing*, similar to duck typing but checked at compile time.

```go
type Stringer interface {
    String() string
}

type Point struct{ X, Y int }
func (p Point) String() string { return fmt.Sprintf("(%d,%d)", p.X, p.Y) }

// Point satisfies Stringer automatically — no declaration needed.
var s Stringer = Point{1, 2}
```

### The empty interface & `any`

`interface{}` (aliased as `any` since Go 1.18) is satisfied by every type — Go's escape hatch for "value of unknown type," used before generics for containers, and still used for truly heterogeneous data (JSON blobs, `fmt.Println` args).

```go
func describe(i any) {
    fmt.Printf("(%v, %T)\n", i, i)
}
```

### Type assertions & type switches

```go
var i any = "hello"

s, ok := i.(string) // "comma ok" form — safe, ok=false if assertion fails
s2 := i.(string)     // panics if i is not a string

switch v := i.(type) {
case int:
    fmt.Println("int", v)
case string:
    fmt.Println("string", v)
default:
    fmt.Println("other")
}
```

### Interface composition

Small, composable interfaces are idiomatic Go — the standard library's `io.Reader`/`io.Writer` are the canonical example (each a single method):

```go
type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }
type ReadWriter interface {
    Reader
    Writer
}
```

**"Accept interfaces, return structs"** is the classic Go API guideline: function parameters should be the narrowest interface that satisfies the need (maximizing caller flexibility and testability); return concrete types (giving callers full access, and letting *them* narrow to an interface if they want).

### `nil` interfaces — the classic gotcha

An interface value is `nil` only if **both** its type and value are nil. A nil pointer wrapped in an interface is **not** a nil interface:

```go
type MyError struct{}
func (e *MyError) Error() string { return "boom" }

func mayFail() error {
    var e *MyError = nil
    if false {
        e = &MyError{}
    }
    return e // returns a non-nil interface wrapping a nil *MyError!
}

err := mayFail()
fmt.Println(err == nil) // false! err has type *MyError, value nil — but the interface itself isn't nil
```

**Fix:** return `nil` literally when there's no error, don't return a typed nil pointer through an `error`-returning function.

---

## 17. Generics (Type Parameters)

Introduced in Go 1.18 (2022). Generics let functions and types be parameterized over types, constrained by an interface listing allowed underlying types/methods.

```go
func Map[T, U any](s []T, f func(T) U) []U {
    result := make([]U, len(s))
    for i, v := range s {
        result[i] = f(v)
    }
    return result
}

squares := Map([]int{1, 2, 3}, func(n int) int { return n * n })
strs := Map([]int{1, 2, 3}, func(n int) string { return strconv.Itoa(n) })
```

### Constraints

A constraint is an interface, optionally listing a union of permitted underlying types via `|`:

```go
type Number interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float32 | ~float64
}
// The ~ means "any type whose underlying type is T" (so custom named types based on int also qualify)

func Sum[T Number](nums []T) T {
    var total T
    for _, n := range nums {
        total += n
    }
    return total
}
```

The standard library's `constraints` concepts live in `golang.org/x/exp/constraints` historically, and core comparable/ordered support is built in via the predeclared `comparable` constraint:

```go
func Contains[T comparable](s []T, target T) bool {
    for _, v := range s {
        if v == target {
            return true
        }
    }
    return false
}
```

### Generic types

```go
type Stack[T any] struct {
    items []T
}

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }
func (s *Stack[T]) Pop() (T, bool) {
    var zero T
    if len(s.items) == 0 {
        return zero, false
    }
    n := len(s.items) - 1
    v := s.items[n]
    s.items = s.items[:n]
    return v, true
}

s := &Stack[int]{}
s.Push(1)
s.Push(2)
v, _ := s.Pop() // 2
```

**When to use generics:** container types (stacks, sets, trees), algorithms that operate identically across types (`Map`/`Filter`/`Reduce`, `Sum`, `Max`), and to eliminate `interface{}` + type-assertion boilerplate/runtime panics. **Avoid over-genericizing:** if a function only ever operates on one concrete type in practice, a generic signature adds indirection without benefit. Go's stdlib `slices` and `maps` packages (Go 1.21+) are generic and should be preferred over hand-rolled equivalents.

---

## 18. Error Handling

Go models failure as ordinary values via the built-in `error` interface — no exceptions, no hidden control flow:

```go
type error interface {
    Error() string
}
```

```go
func divide(a, b int) (int, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}

result, err := divide(10, 0)
if err != nil {
    log.Fatal(err)
}
```

### Wrapping errors (`%w`, `errors.Is`, `errors.As`, `errors.Unwrap`)

Since Go 1.13, errors can be wrapped to preserve a causal chain while adding context:

```go
var ErrNotFound = errors.New("not found")

func fetch(id string) error {
    if !exists(id) {
        return fmt.Errorf("fetch %s: %w", id, ErrNotFound) // %w wraps
    }
    return nil
}

err := fetch("123")
if errors.Is(err, ErrNotFound) {  // walks the wrap chain, unlike err == ErrNotFound
    fmt.Println("not found!")
}

var myErr *MyCustomError
if errors.As(err, &myErr) {       // finds the first wrapped error matching this concrete type
    fmt.Println(myErr.Code)
}
```

### Custom error types

```go
type ValidationError struct {
    Field string
    Msg   string
}
func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Msg)
}
```

### `errors.Join` (Go 1.20+) — combine multiple errors

```go
err := errors.Join(err1, err2, err3) // nil errors are dropped; errors.Is/As traverse all of them
```

**When to use `error` vs `panic`:** errors represent expected, recoverable failure conditions (bad input, network timeout, not-found) that callers should handle explicitly. `panic` is reserved for programmer errors and truly unrecoverable states (index out of range, nil dereference, invariant violations) — see §19.

---

## 19. Defer, Panic & Recover

### `defer`

Schedules a function call to run when the surrounding function returns — LIFO order, arguments evaluated immediately (at the `defer` statement), but the call executes later.

```go
func readFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close() // guaranteed cleanup, runs even if a later return/panic happens
    // ... use f ...
    return nil
}
```

```go
func example() {
    for i := 0; i < 3; i++ {
        defer fmt.Println(i) // prints 2, 1, 0 — LIFO, and i is captured at defer-time (each is its own snapshot per iteration since Go 1.22 loop semantics; also true here regardless since it's an argument, evaluated immediately)
    }
}
```

Deferred functions can **modify named return values** — this is how `recover()` typically converts a panic into a returned error:

```go
func safeDivide(a, b int) (result int, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered: %v", r)
        }
    }()
    result = a / b // panics on b == 0
    return
}
```

### `panic` & `recover`

`panic` immediately stops normal execution of the current function, runs deferred calls up the call stack, and (unless recovered) crashes the program with a stack trace.

`recover()` only has effect when called **directly inside a deferred function**; it stops the panic's unwinding and returns the value passed to `panic`.

```go
func mustPositive(n int) int {
    if n < 0 {
        panic(fmt.Sprintf("expected positive, got %d", n))
    }
    return n
}
```

**When to use:** `panic` for programmer errors and invariant violations within a single process (e.g., a library that validates its own preconditions), never as a substitute for normal error returns across API boundaries. A well-behaved library should recover from its own internal panics at the boundary and convert them to `error` returns rather than letting a panic propagate to the caller (see `encoding/json`'s internal use of panic/recover, for example).

---

## 20. Goroutines & the Go Scheduler

A goroutine is a function executing concurrently with the rest of the program, scheduled by the Go runtime rather than the OS — vastly cheaper than an OS thread (starts at ~2KB stack, grows/shrinks dynamically, vs. ~1-8MB fixed for OS threads).

```go
go doWork()             // starts a goroutine, returns immediately
go func() {
    fmt.Println("running concurrently")
}()
```

### The GMP Model

The Go scheduler multiplexes goroutines onto OS threads using three entities:

- **G** (Goroutine): the unit of work — function + stack + state.
- **M** (Machine): an OS thread.
- **P** (Processor): a scheduling context; there are `GOMAXPROCS` Ps, each holding a local run queue of Gs. An M must hold a P to execute Go code.

This M:N scheduling means thousands or millions of goroutines can run on a handful of OS threads. When a goroutine blocks on a **network-related syscall**, the runtime's integrated netpoller allows the P to be handed to another M so other goroutines keep running — blocking I/O doesn't block an entire OS thread the way it would in a naive thread-per-request model.

`GOMAXPROCS` controls the number of Ps (effectively, how many goroutines run truly in parallel) — defaults to `runtime.NumCPU()`.

**Common pitfalls:**

- **Forgetting to synchronize:** the `main()` goroutine doesn't wait for spawned goroutines to finish; the program exits when `main` returns, killing all outstanding goroutines. Use `sync.WaitGroup` or channels to wait.
- **Goroutine leaks:** a goroutine blocked forever on a channel send/receive that nobody will ever satisfy never gets garbage collected (goroutines aren't GC'd like objects — they must exit their function to be reclaimed).

```go
var wg sync.WaitGroup
for i := 0; i < 5; i++ {
    wg.Add(1)
    go func(n int) {
        defer wg.Done()
        fmt.Println(n)
    }(i)
}
wg.Wait()
```

---

## 21. Channels & `select`

A channel is a typed, synchronized conduit for communicating between goroutines — Go's embodiment of "Do not communicate by sharing memory; instead, share memory by communicating."

```go
ch := make(chan int)      // unbuffered — send blocks until a receiver is ready
ch := make(chan int, 10)  // buffered — send blocks only when the buffer is full

ch <- 42       // send
v := <-ch      // receive
v, ok := <-ch  // ok=false if channel is closed and drained

close(ch)      // signals no more values will be sent; receiving from a closed channel yields zero values immediately (with ok=false)
```

**Directional channel types** restrict a channel parameter to send-only or receive-only, enforced at compile time:

```go
func producer(out chan<- int) { // send-only
    for i := 0; i < 5; i++ { out <- i }
    close(out)
}
func consumer(in <-chan int) { // receive-only
    for v := range in {        // ranging over a channel reads until it's closed
        fmt.Println(v)
    }
}
```

### `select`

Waits on multiple channel operations simultaneously, proceeding with whichever is ready first (randomly among ties):

```go
select {
case v := <-ch1:
    fmt.Println("from ch1:", v)
case v := <-ch2:
    fmt.Println("from ch2:", v)
case ch3 <- 42:
    fmt.Println("sent to ch3")
case <-time.After(time.Second):
    fmt.Println("timeout")
default:
    fmt.Println("nothing ready — non-blocking")
}
```

**Common concurrency patterns:**

- **Fan-out/fan-in:** spawn N worker goroutines reading from a shared input channel, merge results on an output channel.
- **Pipeline:** chain of goroutines, each stage reads from an input channel and writes to an output channel.
- **Done/cancellation channel:** `done := make(chan struct{})`; `close(done)` to broadcast cancellation to all listeners (closing, not sending, since every receiver on a closed channel wakes up).
- Prefer `context.Context` (see §24) over raw done-channels in modern code for cancellation + deadlines + value propagation.

**When to use channels vs. mutexes:** channels for orchestrating goroutine lifecycles and passing ownership of data; mutexes for protecting simple shared in-memory state accessed by many goroutines without a natural "pipeline" shape. Rob Pike's guidance: "channels orchestrate, mutexes serialize."

---

## 22. The `sync` Package

### `sync.Mutex` / `sync.RWMutex`

```go
type SafeCounter struct {
    mu sync.Mutex
    n  int
}
func (c *SafeCounter) Inc() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.n++
}
```

`RWMutex` allows multiple concurrent readers OR one writer — use it when reads vastly outnumber writes:

```go
var mu sync.RWMutex
mu.RLock(); defer mu.RUnlock()   // for reads
mu.Lock(); defer mu.Unlock()     // for writes
```

### `sync.WaitGroup`

Waits for a collection of goroutines to finish (see §20 example). `Add` before spawning, `Done` (usually deferred) inside the goroutine, `Wait` blocks until the counter hits zero.

### `sync.Once`

Guarantees a function runs exactly once, even under concurrent calls — the idiomatic way to do lazy, thread-safe singleton initialization:

```go
var once sync.Once
var instance *Singleton

func GetInstance() *Singleton {
    once.Do(func() {
        instance = &Singleton{}
    })
    return instance
}
```

### `sync.Map`

A concurrent map optimized for two specific access patterns: (1) keys written once and read many times, or (2) many goroutines each operating on disjoint key sets. For general-purpose concurrent maps, a plain `map` + `Mutex` is usually simpler and faster — `sync.Map`'s API (`Load`, `Store`, `Delete`, `Range`) is intentionally awkward to discourage casual overuse.

### `sync.Pool`

A cache of temporarily unused objects to reduce GC pressure for frequently allocated/discarded objects (e.g., buffers):

```go
var bufPool = sync.Pool{
    New: func() any { return new(bytes.Buffer) },
}
buf := bufPool.Get().(*bytes.Buffer)
buf.Reset()
defer bufPool.Put(buf)
```

---

## 23. `sync/atomic`

Provides lock-free primitives for simple counters/flags, operating directly on memory via CPU atomic instructions — cheaper than a mutex for single-variable updates.

```go
var counter atomic.Int64 // typed atomic wrapper (Go 1.19+) — preferred modern API

counter.Add(1)
v := counter.Load()
counter.Store(0)
counter.CompareAndSwap(0, 100) // CAS — the building block of lock-free algorithms
```

Older code uses function-based atomics on plain integers (`atomic.AddInt64(&n, 1)`) — the typed wrapper API is preferred in new code because it prevents accidentally reading/writing the variable non-atomically elsewhere.

**When to use atomics vs. mutex:** atomics for a single primitive counter/flag/pointer; a mutex once you need to update *multiple* related fields consistently (atomics can't express multi-field invariants).

---

## 24. `context` Package

`context.Context` carries deadlines, cancellation signals, and request-scoped values across API boundaries and between goroutines — the standard mechanism for cooperative cancellation in Go.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel() // always call cancel to release resources, even if the operation finished normally

go func() {
    select {
    case <-ctx.Done():
        fmt.Println("cancelled:", ctx.Err())
    case <-time.After(5 * time.Second):
        fmt.Println("work finished")
    }
}()

cancel() // triggers ctx.Done()
```

```go
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel()

ctx, cancel := context.WithDeadline(context.Background(), someTime)
defer cancel()

ctx = context.WithValue(ctx, key{}, "some-value") // request-scoped values only — NOT for optional params
```

**Conventions (important — these are load-bearing idioms, not suggestions):**

- `ctx` is always the **first parameter**, named `ctx`, never stored inside a struct.
- Only `context.Background()` (root, for main/tests/top-level) or `context.TODO()` (placeholder, "I haven't decided yet") create root contexts — everything else derives from a parent.
- `context.WithValue` should carry request-scoped metadata (trace IDs, auth tokens) — **not** optional function parameters; overusing it turns your code's dependencies invisible.
- Always call the returned `cancel()` function (typically via `defer`), even on success, to release the context's internal timer/resources.

---

## 25. Packages, Modules & Visibility

### Modules

A module is the unit of versioning and dependency management, declared by a `go.mod` file at its root:

```
module github.com/yourname/yourproject

go 1.22

require (
    github.com/some/dependency v1.4.0
)
```

- `go.mod` pins the module path, the minimum Go language version, and direct/indirect dependencies.
- `go.sum` records cryptographic checksums of every dependency (including transitive ones) for reproducible, tamper-evident builds.
- Go uses **Minimal Version Selection (MVS)**: given a build's full dependency graph, it picks the *minimum* version satisfying all `require` directives — not the latest, unlike npm's typical resolution — for maximally reproducible builds.
- Semantic import versioning: a module at major version ≥2 must include the version in its import path (`github.com/user/repo/v2`), because Go treats different major versions as different packages entirely (no structural compatibility is assumed across majors).

### Visibility

Go's *only* visibility mechanism is capitalization of the identifier's first letter — no `public`/`private`/`protected` keywords:

```go
package account

type Account struct {
    Balance float64 // exported — accessible from other packages
    pin     string  // unexported — accessible only within package `account`
}

func NewAccount() *Account { return &Account{} } // exported constructor
func (a *Account) validate() bool { return true } // unexported helper method
```

This applies at every level: package-level functions/vars/consts, struct fields, interface methods, and type names. Unexported identifiers are invisible outside the declaring package — full stop, no exceptions, no `friend` classes.

---

## 26. Internal Packages

Any package whose import path contains a directory literally named `internal` is importable **only** by code rooted at the parent of that `internal` directory — enforced by the compiler, not just convention.

```
myproject/
├── internal/
│   └── auth/
│       └── auth.go        // importable only within myproject/...
├── pkg/
│   └── public/
│       └── public.go
└── cmd/
    └── server/
        └── main.go         // CAN import myproject/internal/auth
```

```go
// This compiles fine — main.go is under myproject/, the same tree as internal/
import "github.com/yourname/myproject/internal/auth"
```

```go
// From an entirely different module/repo — this is a COMPILE ERROR:
// "use of internal package ... not allowed"
import "github.com/yourname/myproject/internal/auth"
```

**When to use:** any package that's an implementation detail you want to freely refactor without worrying about external consumers depending on it — database access layers, internal utility helpers, generated code, anything not meant to be part of your public API surface. This is Go's primary mechanism for enforcing "public API vs. implementation detail" at the tooling level rather than relying on documentation/convention alone (unlike, say, Python's leading-underscore convention which tooling doesn't enforce).

---

## 27. `go:embed` — Embedded Filesystems

The `embed` package (Go 1.16+) lets you compile static files (templates, migrations, static web assets, certificates) directly into the binary, eliminating runtime dependencies on the filesystem layout.

```go
package main

import (
    "embed"
    "io/fs"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed config.json
var configData []byte  // single file into a byte slice

//go:embed version.txt
var version string      // single file into a string
```

**Key rules:**

- The `//go:embed` directive must be **immediately above** a package-level variable declaration of type `string`, `[]byte`, or `embed.FS`, with no blank line between the comment and the declaration.
- Must `import "embed"` even if you never reference the package name directly (needed to enable the compiler directive) — the blank import trick doesn't apply here; a *named but unused-looking* import is required and the compiler special-cases it.
- Glob patterns are supported (`*.html`), and directories are embedded recursively.
- By default, files/directories starting with `.` or `_` are excluded from a directory-embed pattern unless explicitly matched (`all:static` includes them).
- `embed.FS` implements `fs.FS`, so it composes with the whole `io/fs` ecosystem (`http.FileServer(http.FS(staticFS))`, `template.ParseFS`, etc.):

```go
tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))

http.Handle("/static/", http.FileServer(http.FS(staticFS)))
```

**When to use:** shipping a single self-contained binary that includes its web UI assets, default config, database migration SQL files, or reference data — a huge deployment simplification versus shipping a binary plus a directory of loose files that must stay in sync.

---

## 28. `go generate` — Code Generation

`go generate` is a convention, not a build step: running `go generate ./...` scans source files for specially formatted comments and executes the command each one specifies. It is never run automatically by `go build`/`go test` — it's a deliberate, explicit, manual (or CI-scripted) step.

```go
//go:generate stringer -type=Weekday
type Weekday int

const (
    Sunday Weekday = iota
    Monday
)
```

Running `go generate ./...` executes `stringer -type=Weekday` in that file's directory, which produces a `weekday_string.go` file implementing `String()` for the `Weekday` type by inspecting the `const` block — turning the `iota` pattern from §3 into a full `fmt.Stringer` implementation without hand-writing the switch statement.

Common generators:

- `stringer` — generates `String()` methods for enum-like `iota` constants.
- `mockgen` / `mockery` — generates interface mocks for testing.
- `protoc-gen-go` — generates Go structs from `.proto` schema files.
- Custom generators for wire formats, SQL query builders (`sqlc`), GraphQL resolvers, etc.

**Directive syntax rule:** `//go:generate` (no space after `//`) followed by a shell command; must appear as its own comment line, and is typically placed directly above the type/declaration it concerns for discoverability, though the tool itself only cares that the comment exists in a scanned `.go` file, not its exact position.

**When to use:** any case where writing boilerplate by hand is repetitive and mechanically derivable from other source (enum stringification, mocks, schema-derived types) — codifies "generate once, commit the output, regenerate on change" rather than computing it at runtime via reflection (trading a small maintenance step for zero runtime cost and full static-analysis/IDE support on the generated code).

---

## 29. Build Constraints (`//go:build`)

Build constraints (tags) conditionally include/exclude a file from compilation based on OS, architecture, Go version, or custom tags passed via `-tags`.

```go
//go:build linux && amd64

package mypkg
// This file only compiles on linux/amd64
```

```go
//go:build (linux || darwin) && !windows
```

Filename-based constraints are equivalent and automatic — no comment needed:

- `foo_linux.go` — only compiles on GOOS=linux.
- `foo_amd64.go` — only compiles on GOARCH=amd64.
- `foo_linux_amd64.go` — both.
- `foo_test.go` — only included when running tests.

Custom build tags, useful for optional features or integration-test gating:

```go
//go:build integration

package db
// Only compiled when built with: go test -tags=integration ./...
```

**Rules:**

- The `//go:build` line must appear before the `package` clause, with a blank line separating it from `package` (a lint/vet rule, not strictly a parse requirement, but treated as one in practice).
- The older `// +build` syntax (pre Go 1.17) is deprecated in favor of `//go:build`; `gofmt` auto-generates the old-style comment alongside the new one for backward tool compatibility, but you should only write `//go:build` yourself.

**When to use:** platform-specific implementations behind a common interface (e.g., a `Signal` type that behaves differently on Unix vs. Windows), gating expensive integration/e2e tests behind an explicit flag so `go test ./...` stays fast by default, or maintaining alternate implementations for different Go versions during a migration window.

---

## 30. Reflection (`reflect`)

Reflection lets a program inspect and manipulate types and values at runtime — the mechanism underlying `encoding/json`, `fmt`, struct-tag validators, and ORMs.

```go
v := reflect.ValueOf(42)
t := reflect.TypeOf(42)
fmt.Println(v.Kind())  // reflect.Int
fmt.Println(t.Name())  // "int"

s := reflect.ValueOf(&someStruct).Elem() // Elem() dereferences a pointer Value
for i := 0; i < s.NumField(); i++ {
    field := s.Type().Field(i)
    value := s.Field(i)
    fmt.Printf("%s (%s) = %v\n", field.Name, field.Type, value.Interface())
}
```

Mutating a value via reflection requires an **addressable, settable** `Value` — obtained via `Elem()` on a pointer, never on a value passed by copy:

```go
x := 10
v := reflect.ValueOf(&x).Elem()
v.SetInt(20) // x is now 20
```

**Laws of reflection (Rob Pike's summary):**
1. Reflection goes from interface value to reflection object (`ValueOf`/`TypeOf`).
2. Reflection goes from reflection object back to an interface value (`Interface()`).
3. To modify a reflection object, the underlying value must be settable (addressable).

**Trade-offs:** reflection bypasses compile-time type checking (errors surface as runtime panics), is significantly slower than direct field access (typically 10-100x), and produces code that's harder for readers and static analysis tools to follow. **Use it only when genuinely needed** — generic serialization, plugin systems, generic validation frameworks — and prefer generics (§17) for anything where the set of types is known and closed at compile time.

---

## 31. Range-over-Func Iterators (Go 1.23+)

Go 1.23 extended `range` to accept **functions** matching one of two shapes, enabling custom, lazy, potentially infinite iterator types without allocating an intermediate slice:

```go
type Seq[V any] func(yield func(V) bool)
type Seq2[K, V any] func(yield func(K, V) bool)
```

```go
func Count(n int) func(yield func(int) bool) {
    return func(yield func(int) bool) {
        for i := 0; i < n; i++ {
            if !yield(i) {
                return // consumer used `break` — stop producing
            }
        }
    }
}

for v := range Count(5) {
    fmt.Println(v) // 0, 1, 2, 3, 4
    if v == 2 {
        break // signals the iterator function to stop via yield returning false
    }
}
```

This underlies the new `iter.Seq[V]` / `iter.Seq2[K, V]` types and the Go 1.23 standard-library iterator helpers in `slices` (`slices.All`, `slices.Values`) and `maps` (`maps.Keys`, `maps.Values`):

```go
for i, v := range slices.All(mySlice) { /* i=index, v=value, no manual indexing needed */ }
for k := range maps.Keys(myMap) { /* iterate just keys */ }
```

**When to use:** building your own lazy sequence types (tree traversal, paginated API result iteration, filtering pipelines) where you want `range` ergonomics without materializing a full slice upfront — particularly valuable for large or infinite sequences.

---

## 32. Compiler Directives

Beyond `//go:build`, `//go:generate`, and `//go:embed`, Go recognizes several low-level compiler directives — mostly relevant to standard-library/runtime code or serious performance work, and generally **not** for everyday application code:

```go
//go:noinline
func expensiveButMustNotInline() { /* ... */ }
// Prevents the compiler from inlining this function — useful for benchmarking
// or when inlining would break certain debugging/profiling assumptions.

//go:noescape
func rawSyscall(...) // asserts to the compiler that arguments don't escape to the heap
// (used with assembly-implemented functions the escape analyzer can't see into)

//go:linkname localname importpath.name
// Links a local (often unexported) symbol to a symbol in another package —
// a deliberate escape hatch from Go's visibility rules, used sparingly in the
// standard library itself and by a few low-level libraries; considered fragile
// and explicitly unsupported for general application code.

//go:norace
// Excludes a function from race detector instrumentation.
```

**When to use:** almost never in application code. These exist for the standard library, runtime, and specialized low-level libraries (crypto, sync primitives) that need to bypass normal compiler behavior for correctness or performance reasons that don't apply to typical business logic.

---

## 33. Escape Analysis & the Go Memory Model

### Escape analysis

The compiler decides whether each value can live on the stack (fast allocate/free, no GC involvement) or must be allocated on the heap (if its lifetime or size can't be proven bounded to the current function call) — this is fully automatic; Go has no `stack`/`heap` keywords.

A value "escapes to the heap" typically when:
- A pointer to it is returned from the function.
- It's stored in a struct/slice/map/channel/interface that outlives the function.
- Its size is not known at compile time, or it's simply too large for the stack.
- It's captured by a closure that outlives the function (see §11).

```go
func newInt() *int {
    x := 42
    return &x // x escapes to the heap — its stack frame won't exist after return
}
```

Inspect escape decisions with:

```bash
go build -gcflags="-m" ./...
```

**Why this matters practically:** minimizing heap escapes reduces GC pressure. Passing large structs by value when the compiler can keep them stack-allocated is sometimes *faster* than passing a pointer, contrary to intuition — profile before assuming pointers are always cheaper.

### The Go Memory Model

Defines when a read in one goroutine is *guaranteed* to observe a write from another goroutine — without proper synchronization, there is **no guarantee**, even if the write "happened first" in wall-clock time, because of compiler/CPU reordering and per-core caching.

Synchronization primitives that establish a "happens-before" relationship: channel send/receive, `sync.Mutex` lock/unlock, `sync.WaitGroup`, `sync.Once`, and the `sync/atomic` operations. Sharing memory across goroutines without one of these is a **data race** — undefined behavior, detectable at runtime with:

```bash
go test -race ./...
go run -race main.go
```

**When it matters:** any time two or more goroutines read/write the same variable without a channel, mutex, or atomic guarding it — always run `-race` in CI for concurrent code; it catches real bugs that pass every functional test.

---

## 34. Testing, Benchmarking & Fuzzing

Go has first-class testing built into the toolchain — no external framework required (`testing` package + `go test`).

### Table-driven tests (the dominant Go idiom)

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 2, 3, 5},
        {"negative", -1, -1, -2},
        {"zero", 0, 0, 0},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.expected {
                t.Errorf("Add(%d,%d) = %d; want %d", tt.a, tt.b, got, tt.expected)
            }
        })
    }
}
```

`t.Run` creates a named subtest (shows as `TestAdd/positive` in output), runnable in isolation (`go test -run TestAdd/positive`), and subtests can run in parallel via `t.Parallel()`.

### Benchmarks

```go
func BenchmarkAdd(b *testing.B) {
    for i := 0; i < b.N; i++ { // b.N auto-tuned by the framework for statistically stable timing
        Add(2, 3)
    }
}
```

```bash
go test -bench=. -benchmem ./...  # -benchmem shows allocations/op
```

### Fuzzing (Go 1.18+)

Generates random/mutated inputs to find edge cases/crashes automatically, seeded by an initial corpus:

```go
func FuzzReverse(f *testing.F) {
    f.Add("hello") // seed corpus
    f.Fuzz(func(t *testing.T, s string) {
        rev := Reverse(s)
        if Reverse(rev) != s {
            t.Errorf("Reverse(Reverse(%q)) != %q", s, s)
        }
    })
}
```

```bash
go test -fuzz=FuzzReverse -fuzztime=30s
```

### Other testing tools

- **`testing.T.Cleanup`**: registers cleanup functions run after the test (like `defer`, but composes cleanly across helper functions).
- **`httptest`**: in-memory HTTP server/recorder for testing handlers without a real network socket.
- **Golden files**: comparing test output against a checked-in reference file, commonly regenerated via a `-update` flag.
- **`go vet`**: static analysis catching suspicious constructs (`Printf` format mismatches, unreachable code, struct tag typos) — run automatically by `go test`.

---

## 35. The Blank Identifier `_`

`_` discards a value that must be present syntactically but is genuinely unused — it's not a variable, has no storage, and can be "assigned to" any number of times.

```go
_, err := someFunc()              // discard a return value
for _, v := range items { }        // discard the index
_ = unusedButRequiredParam          // silence "declared and not used" (rare, usually a smell)
var _ io.Reader = (*MyType)(nil)   // COMPILE-TIME interface-satisfaction check, no runtime cost
import _ "github.com/lib/pq"       // blank import for side effects only (driver registration)
```

The `var _ SomeInterface = (*Concrete)(nil)` pattern is a widely used idiom to assert, at compile time, that a type implements an interface — without creating any usable variable, and failing the build immediately (not at first use) if the type ever stops satisfying the interface.

---

## 36. Labels, `goto`, `break`, `continue`

Go supports labeled `break`/`continue` to control nested loops precisely — a common and legitimate substitute for `goto`-based loop exits found in C:

```go
outer:
for i := 0; i < 3; i++ {
    for j := 0; j < 3; j++ {
        if j == 1 {
            continue outer // continues the OUTER loop, not the inner one
        }
        if i == 2 {
            break outer // breaks out of BOTH loops
        }
        fmt.Println(i, j)
    }
}
```

`goto` exists but is rare and restricted (cannot jump into a block or over a variable declaration that's still in scope at the target):

```go
func retryLoop() {
    i := 0
retry:
    if i < 3 {
        fmt.Println("attempt", i)
        i++
        goto retry
    }
}
```

**When to use:** labeled break/continue for cleanly exiting/skipping nested loops (very idiomatic, common in parser/scanner code). Plain `goto` is rare in idiomatic Go — mostly seen in generated code or very specific control-flow simplifications (e.g., avoiding deeply nested error-handling `if`s in older code, largely superseded by early-return style).

---

## 37. `init()` Functions

Any file may declare one or more `func init()` — run automatically before `main()`, after all package-level variable initializers in that file, in file-name order within a package, and in dependency order across packages (all of a package's dependencies' `init`s run before its own).

```go
package config

var settings map[string]string

func init() {
    settings = loadDefaults()
}

func init() { // multiple init() per file/package is legal
    validateEnvironment()
}
```

**Order guarantees:**
1. Package-level `var` initializers run first (in dependency order among the variables themselves).
2. Then `init()` functions run, in the order they appear across the files of a package (files are processed in the order presented to the compiler, typically alphabetical).
3. Imported packages are always fully initialized (vars + all their `init`s) before the importing package's own initialization begins.

**When to use — sparingly:** registering plugins/drivers (`database/sql` driver registration via blank import + `init()` is the canonical example), one-time validation of required environment/config at startup. **Avoid overusing `init()`** for anything beyond registration-style side effects — it runs implicitly with no caller-controlled ordering or parameters, makes testing harder (you can't easily avoid or parameterize it), and hides real dependencies that would be clearer as explicit constructor calls in `main()`.

---

## 38. Idioms & Best Practices Checklist

A distilled list of what separates idiomatic Go from "code that happens to compile":

- **Accept interfaces, return structs.** Keep function parameters as narrow interfaces; return concrete types.
- **Errors are values.** Check them immediately (`if err != nil { return err }`), wrap with `%w` for context, never ignore silently (`_ = err` should be rare and deliberate).
- **Handle the "happy path" last, or keep it unindented.** Prefer early returns over deep nesting (`if err != nil { return }` then continue, rather than wrapping the rest of the function in an `else`).
- **Don't panic across API boundaries.** Panics are for the same process's unrecoverable invariant violations, not for expected failure conditions a caller should handle.
- **Keep interfaces small.** One or two methods is typical and idiomatic (`io.Reader`, `sort.Interface`); large interfaces are a design smell.
- **Use `gofmt`/`goimports` always**, and `go vet`/`staticcheck` in CI — formatting is not a style debate in Go, it's automated and non-negotiable.
- **Prefer composition (embedding) over building deep type hierarchies.**
- **Name packages for what they provide, not what they contain** (`net/http`, not `utils` or `common` — avoid generic dumping-ground package names).
- **Zero values should be useful.** Design types so `var t T` is immediately usable without an explicit constructor when possible (`sync.Mutex{}` needs no `New`).
- **Concurrency is not automatically parallelism, and not automatically correct.** Always reason explicitly about what synchronizes what; run `-race` in CI.
- **Document exported identifiers** with a comment starting with the identifier's name (`// Foo does X.`) — this is what `go doc`/pkg.go.dev render, and `golint`/`staticcheck` enforce the convention.
- **Table-driven tests over repeated near-duplicate test functions.**

---

*This reference reflects the Go language through Go 1.23. Language features continue to evolve (e.g., generics arrived in 1.18, loop-variable semantics changed in 1.22, range-over-func arrived in 1.23) — always check the [official Go release notes](https://go.dev/doc/devel/release) for the version you're targeting.*
