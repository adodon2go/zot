// nolint: goerr113
package errors

import (
	"errors"
	"fmt"
)

var (
	ErrBadConfig               = errors.New("config: invalid config")
	ErrRepoNotFound            = errors.New("repository: not found")
	ErrRepoIsNotDir            = errors.New("repository: not a directory")
	ErrRepoBadVersion          = errors.New("repository: unsupported layout version")
	ErrManifestNotFound        = errors.New("manifest: not found")
	ErrBadManifest             = errors.New("manifest: invalid contents")
	ErrBadIndex                = errors.New("index: invalid contents")
	ErrUploadNotFound          = errors.New("uploads: not found")
	ErrBadUploadRange          = errors.New("uploads: bad range")
	ErrBlobNotFound            = errors.New("blob: not found")
	ErrBadBlob                 = errors.New("blob: bad blob")
	ErrBadBlobDigest           = errors.New("blob: bad blob digest")
	ErrUnknownCode             = errors.New("error: unknown error code")
	ErrBadCACert               = errors.New("tls: invalid ca cert")
	ErrBadUser                 = errors.New("ldap: non-existent user")
	ErrEntriesExceeded         = errors.New("ldap: too many entries returned")
	ErrLDAPEmptyPassphrase     = errors.New("ldap: empty passphrase")
	ErrLDAPBadConn             = errors.New("ldap: bad connection")
	ErrLDAPConfig              = errors.New("config: invalid LDAP configuration")
	ErrCacheRootBucket         = errors.New("cache: unable to create/update root bucket")
	ErrCacheNoBucket           = errors.New("cache: unable to find bucket")
	ErrCacheMiss               = errors.New("cache: miss")
	ErrRequireCred             = errors.New("ldap: bind credentials required")
	ErrInvalidCred             = errors.New("ldap: invalid credentials")
	ErrInvalidArgs             = errors.New("cli: Invalid Arguments")
	ErrInvalidFlagsCombination = errors.New("cli: Invalid combination of flags")
	ErrInvalidURL              = errors.New("cli: invalid URL format")
	ErrUnauthorizedAccess      = errors.New("cli: unauthorized access. check credentials")
	ErrCannotResetConfigKey    = errors.New("cli: cannot reset given config key")
	ErrConfigNotFound          = errors.New("cli: config with the given name does not exist")
	ErrNoURLProvided           = errors.New("cli: no URL provided in argument or via config. see 'zot config -h'")
	ErrIllegalConfigKey        = errors.New("cli: given config key is not allowed")
	ErrScanNotSupported        = errors.New("search: scanning of image media type not supported")
	ErrCLITimeout              = errors.New("cli: Query timed out while waiting for results")
	ErrDuplicateConfigName     = errors.New("cli: cli config name already added")
	ErrInvalidRoute            = errors.New("routes: invalid route prefix")
	ErrImgStoreNotFound        = errors.New("routes: image store not found corresponding to given route")
	ErrEmptyValue              = errors.New("cache: empty value")
	ErrEmptyRepoList           = errors.New("search: no repository found")
)

func ErrMetricNotFound(name string) error {
	return fmt.Errorf("metric %s: not found", name)
}

func ErrLabelSize(name string) error {
	return fmt.Errorf("metric %s: label size mismatch", name)
}

func ErrLabelOrder(name string) error {
	return fmt.Errorf("metric %s: label order mismatch", name)
}
