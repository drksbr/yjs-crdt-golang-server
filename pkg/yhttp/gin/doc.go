// Package gin expõe um adapter fino entre `http.Handler` e handlers do
// framework Gin.
//
// O uso principal é adaptar `pkg/yhttp.Server` para rotas Gin sem duplicar a
// lógica de transporte do protocolo Yjs.
package gin
