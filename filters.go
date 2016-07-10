/*
Copyright (c) 2016, Maxim Konakov
All rights reserved.

Redistribution and use in source and binary forms, with or without modification,
are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
   this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.
3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software without
   specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY
OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE,
EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"text/scanner"
)

type filterList struct {
	lineFilters, textFilters []func([]byte) []byte
}

// filter definition reader
func (maker *filterList) add(input io.Reader, name string) (err error) {
	// tokeniser
	tok := makeTokeniser(input, func(t *scanner.Scanner, msg string) {
		msg = fmt.Sprintf("Filter definition from \"%s\", line %d: %s.", name, t.Line, msg)
		err = errors.New(msg)
	})

	// Filter spec format
	//	scope type `match` `replacement`
	// where
	//	scope: 'line' | 'text'
	//	type:	'word' | 'regex'

	// parser
	for t := skipNewLines(tok); t != scanner.EOF; {
		// scope
		if t != scanner.Ident {
			errInvalidToken(tok, "filter scope", t)
			return
		}

		scope := tok.TokenText()

		// filter type
		if t = tok.Scan(); t != scanner.Ident {
			errInvalidToken(tok, "filter type", t)
			return
		}

		filterType := tok.TokenText()

		// regex or word
		if t = tok.Scan(); t != scanner.String {
			errInvalidToken(tok, "regular expression or word", t)
			return
		}

		var match string

		if match, err = strconv.Unquote(tok.TokenText()); err != nil {
			tok.Error(tok, "Regular expression or word string: "+err.Error())
			return
		}

		if len(match) == 0 {
			tok.Error(tok, "Regular expression or word cannot be empty")
			return
		}

		// substitution
		if t = tok.Scan(); t != scanner.String {
			errInvalidToken(tok, "substitution string", t)
			return
		}

		var subst string

		if subst, err = strconv.Unquote(tok.TokenText()); err != nil {
			tok.Error(tok, "Invalid substitution string: "+err.Error())
			return
		}

		// create filter function
		var filterFunc func([]byte) []byte

		switch filterType {
		case "word":
			filterFunc = makeWordFilter([]byte(match), []byte(subst))
		case "regex":
			if re, e := regexp.Compile(match); e != nil {
				tok.Error(tok, e.Error())
				return
			} else {
				filterFunc = makeRegexFilter(re, []byte(subst))
			}
		default:
			tok.Error(tok, "Unknown filter type: "+filterType)
			return
		}

		switch scope {
		case "line":
			maker.lineFilters = append(maker.lineFilters, filterFunc)
		case "text":
			maker.textFilters = append(maker.textFilters, filterFunc)
		default:
			tok.Error(tok, "Unknown filter scope: "+scope)
			return
		}

		// newline or EOF
		switch t = tok.Scan(); t {
		case scanner.EOF:
			// nothing to do
		case '\n':
			t = skipNewLines(tok)
		default:
			errInvalidToken(tok, "newline", t)
			return
		}
	}

	return
}

func makeTokeniser(input io.Reader, errFunc func(*scanner.Scanner, string)) *scanner.Scanner {
	tok := new(scanner.Scanner).Init(input)

	tok.Mode = scanner.SkipComments | scanner.ScanComments | scanner.ScanIdents | scanner.ScanStrings | scanner.ScanRawStrings
	tok.Whitespace = 1<<'\t' | 1<<'\r' | 1<<' '
	tok.Error = errFunc
	return tok
}

func errInvalidToken(tok *scanner.Scanner, msg string, t rune) {
	tok.Error(tok, fmt.Sprintf("Expected %s, but found %s", msg, strconv.Quote(tok.TokenText())))
}

func skipNewLines(tok *scanner.Scanner) (t rune) {
	for t = tok.Scan(); t == '\n'; t = tok.Scan() {
		// empty
	}

	return
}

func makeWordFilter(match, subst []byte) func([]byte) []byte {
	return func(s []byte) []byte {
		return bytes.Replace(s, match, subst, -1)
	}
}

func makeRegexFilter(re *regexp.Regexp, subst []byte) func([]byte) []byte {
	return func(s []byte) []byte {
		return re.ReplaceAll(s, subst)
	}
}
