package wallet

import "errors"

var NoPrivateKeySetError = errors.New("No private key is set.")
var NoSaltSetError = errors.New("No salt is set.")
var NoEncryptedKeyFound = errors.New("No encrypted key found.")
var InvalidTypeSpecified = errors.New("Invalid account type specified.")
