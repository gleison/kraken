package domain

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func validateRequiredText(field, value string) error {
	if strings.TrimSpace(value) == "" || strings.ContainsRune(value, 0) || !utf8.ValidString(value) {
		return fmt.Errorf("%s: nonblank UTF-8 text without NUL is required", field)
	}
	return nil
}

func validateIdentifier(field, value string) error {
	if value == "" || !utf8.ValidString(value) || strings.ContainsAny(value, "/\\") || strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) >= 0 {
		return fmt.Errorf("%s: identifier must be nonempty and contain no whitespace, controls or slashes", field)
	}
	return nil
}

func validateTextEntries(field string, values []string) error {
	for index, value := range values {
		if err := validateRequiredText(fmt.Sprintf("%s[%d]", field, index), value); err != nil {
			return err
		}
	}
	return nil
}

func registerUniqueIdentifier(registeredIdentifiers map[string]bool, field, identifier string) error {
	if err := validateIdentifier(field, identifier); err != nil {
		return err
	}
	if registeredIdentifiers[identifier] {
		return fmt.Errorf("%s: duplicate identifier %q", field, identifier)
	}
	registeredIdentifiers[identifier] = true
	return nil
}

func validateReferences(field string, references []string, knownIdentifiers map[string]bool) error {
	seenReferences := make(map[string]bool)
	for _, identifier := range references {
		if err := registerUniqueIdentifier(seenReferences, field, identifier); err != nil {
			return err
		}
		if !knownIdentifiers[identifier] {
			return fmt.Errorf("%s: unknown reference %q", field, identifier)
		}
	}
	return nil
}

func validateRequiredReferences(field string, references []string, knownIdentifiers map[string]bool) error {
	if len(references) == 0 {
		return fmt.Errorf("%s: at least one reference is required", field)
	}
	return validateReferences(field, references, knownIdentifiers)
}

func validateContracts(contracts ...Validatable) error {
	for _, contract := range contracts {
		if err := contract.Validate(); err != nil {
			return err
		}
	}
	return nil
}
