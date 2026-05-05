# Smoke Tests

Checklist manual curto para validar as áreas alteradas recentemente sem depender de uma suíte e2e completa.

## 1. Shell da nota

1. Abrir uma nota simples em desktop.
2. Confirmar que o header mostra título, status de sync e ações sem quebrar.
3. Abrir `Subdocs`, `Arquivos` e `Áudio`.
4. Confirmar que no desktop o painel abre em coluna lateral e não cobre todo o editor.
5. Repetir em viewport mobile.
6. Confirmar que o painel abre como bottom sheet e fecha corretamente.

## 2. Home

1. Abrir `/` em desktop e mobile.
2. Confirmar que o menu mobile expande e recolhe.
3. Confirmar que hero, métricas, cards de recursos e CTA final mantêm espaçamento consistente.
4. Confirmar que não há sobreposição ruim do `HeroTypingAnimation` no mobile.

## 3. Segurança de nota

1. Abrir uma nota `public`.
2. Abrir uma nota `public-readonly` sem PIN e confirmar modo leitura.
3. Desbloquear com PIN e confirmar que a edição volta.
4. Abrir uma nota `private` e confirmar que o acesso exige PIN.
5. Validar que anexos, áudio e versões da nota protegida não abrem sem acesso autorizado.

## 4. Checklist v2

1. Abrir uma checklist antiga e confirmar que os itens legados aparecem normalmente.
2. Criar uma tarefa raiz.
3. Criar uma subtarefa.
4. Indentar e desindentar uma linha.
5. Recolher e expandir um ramo.
6. Marcar uma subtarefa e confirmar que o pai fica `mixed` até que todas as filhas estejam concluídas.
7. Editar uma linha com:
   `Enter`: cria irmão
   `Ctrl/Cmd+Enter`: cria filho
   `Tab` e `Shift+Tab`: muda nível
8. Reabrir a mesma nota e confirmar que a árvore foi persistida.

## 5. Desenho

1. Abrir uma nota de desenho vazia.
2. Criar pelo menos dois elementos em camadas diferentes.
3. Alterar cor de fundo e mover o viewport.
4. Inserir uma imagem ou outro asset que gere `files` na cena.
5. Reabrir a nota no mesmo dispositivo e confirmar:
   `elements`
   ordem visual das camadas
   `viewBackgroundColor`
   viewport
   asset carregado
6. Abrir a mesma nota em outro dispositivo e confirmar os mesmos pontos.
7. Excluir um elemento, reabrir e confirmar que ele não reaparece.

## 6. Versões

1. Criar uma versão manual de:
   texto
   markdown
   checklist
   kanban
   desenho
2. Alterar o conteúdo em cada tipo.
3. Restaurar a versão anterior.
4. Confirmar que o restore respeita o tipo real da nota e não trata tudo como `Y.Text`.

## 7. Persistência e sync

1. Editar uma nota em um cliente.
2. Fechar a aba.
3. Reabrir em outro cliente sem depender de um terceiro dispositivo para repopular.
4. Repetir com checklist e desenho.
5. Se houver falha, coletar:
   tipo da nota
   se havia asset/anexo
   horário aproximado
   logs de `token` e `flush`
