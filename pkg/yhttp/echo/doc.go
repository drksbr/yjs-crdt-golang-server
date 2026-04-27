// Package echo expõe um adapter fino entre `http.Handler` e handlers do
// framework Echo.
//
// O uso principal é adaptar `pkg/yhttp.Server` para rotas Echo sem duplicar a
// lógica de transporte do protocolo Yjs.
package echo
