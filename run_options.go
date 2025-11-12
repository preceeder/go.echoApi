package echoApi

import (
	"crypto/tls"
	"errors"
)

type RunOption func(*runOptions) error

type runOptions struct {
	tlsCertFile string
	tlsKeyFile  string
	tlsConfig   *tls.Config
}

func (o *runOptions) apply(opts []RunOption) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(o); err != nil {
			return err
		}
	}
	return nil
}

func (o *runOptions) useTLS() bool {
	return o != nil && (o.tlsConfig != nil || (o.tlsCertFile != "" && o.tlsKeyFile != ""))
}

// WithTLSCertificates 使用证书文件配置 TLS。
func WithTLSCertificates(certFile, keyFile string) RunOption {
	return func(o *runOptions) error {
		if certFile == "" || keyFile == "" {
			return errors.New("both certFile and keyFile must be provided when enabling TLS")
		}
		o.tlsCertFile = certFile
		o.tlsKeyFile = keyFile
		return nil
	}
}

// WithTLSConfig 直接注入 *tls.Config。调用方需确保 config 中包含有效的证书。
func WithTLSConfig(cfg *tls.Config) RunOption {
	return func(o *runOptions) error {
		o.tlsConfig = cfg
		return nil
	}
}

