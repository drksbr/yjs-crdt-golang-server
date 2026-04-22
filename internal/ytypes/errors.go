package ytypes

import "errors"

var (
	// ErrInvalidLength sinaliza tentativa de criar uma struct com comprimento vazio.
	ErrInvalidLength = errors.New("ytypes: comprimento invalido")
	// ErrStructOverflow sinaliza que clock+length saiu da faixa uint32 compatível.
	ErrStructOverflow = errors.New("ytypes: struct excede uint32")
	// ErrNilContent impede criar Item sem a descrição mínima do conteúdo.
	ErrNilContent = errors.New("ytypes: content nao pode ser nil")
	// ErrInvalidParentRoot rejeita nomes vazios para referências root-level.
	ErrInvalidParentRoot = errors.New("ytypes: parent root invalido")
)
