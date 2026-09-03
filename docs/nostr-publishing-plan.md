# Plano: Publicacao Nostr por Script Bash

## Objetivo

Criar um script Bash para gerar uma imagem com `fract`, enviar a imagem para
um servidor Blossom e publicar uma nota Nostr assinada por um bunker. A nota
deve conter a URL da imagem, o comando exato de reproducao e o link do projeto:

```text
https://github.com/girino/random-mandelbrot-generator
```

O script deve funcionar em Linux, Git Bash no Windows e WSL. Ele nao deve
armazenar segredos no repositorio nem fazer push Git.

## Entregaveis

- `scripts/publish-nostr.sh`: orquestrador Bash implementado.
- `.env.example`: contrato de configuracao sem valores secretos implementado.
- `README.md`: pre-requisitos, exemplos e limitacoes implementados.
- `scripts/test-publish-nostr.sh`: checks estaticos de sintaxe e composicao.

## Pre-requisitos

- Bash 4 ou superior.
- `fract` disponivel em `PATH`, ou configurado em `FRACT_BIN`.
- `nak` disponivel em `PATH`, ou configurado em `NAK_BIN`.
- `jq` para interpretar os metadados JSON e o Blob Descriptor.
- `sha256sum` para calcular SHA-256.
- Um bunker NIP-46 autorizado a assinar os eventos necessarios.

No Windows, executar pelo Git Bash ou WSL. Caminhos devem ser fornecidos no
formato do ambiente selecionado, por exemplo `/e/girino/Downloads/fract` no
Git Bash ou `/mnt/e/girino/Downloads/fract` no WSL.

## Configuracao

O script recebe `--env ARQUIVO`; sem essa flag, carrega `.env` na raiz do
projeto, independentemente do diretorio atual. O arquivo e local e deve estar
no `.gitignore`.

```dotenv
# Binarios opcionais
FRACT_BIN=fract
NAK_BIN=nak

# Assinatura remota NIP-46. Nunca registrar este valor em logs.
FRACT_BUNKER_URI=bunker://...

# Destinos de publicacao.
FRACT_BLOSSOM_SERVER=https://blossom.primal.net
FRACT_RELAYS=wss://relay.damus.io,wss://nos.lol

# Diretorio para PNG e JSON de cada geracao.
FRACT_OUTPUT_DIR=/caminho/para/fract
```

`FRACT_RELAYS` sera uma lista separada por virgulas. Espacos ao redor dos itens
serao removidos. O script valida URLs e falha antes de gerar ou assinar quando
qualquer variavel obrigatoria estiver ausente.

## Interface Proposta

```bash
scripts/publish-nostr.sh [opcoes do fract random]
```

Exemplo:

```bash
scripts/publish-nostr.sh --seed 42 --size 1080x1080 --random-palette
```

Opcoes proprias do script:

```text
--env ARQUIVO       Arquivo de configuracao, padrao .env
--alt TEXTO         Texto alternativo para a imagem
--content TEXTO     Legenda adicional da nota
--dry-run           Gera e monta a publicacao sem upload nem relay
--keep-failed       Preserva artefatos e resposta HTTP em caso de falha
--help              Exibe uso
```

As demais opcoes sao repassadas a `fract random`.

## Fluxo

1. Carregar e validar o `.env`, usando `umask 077` para arquivos temporarios.
2. Criar um diretorio temporario privado para a execucao.
3. Executar `fract random` com um PNG e JSON de metadados no diretorio
   temporario. O script deve passar `--metadata` explicitamente, pois esse
   arquivo nao e criado sem a flag.
4. Ler o JSON e reconstruir o comando `fract render mandelbrot` a partir dos
   parametros finais. O comando impresso por `fract random` serve apenas como
   diagnostico; os metadados sao a fonte de verdade.
5. Calcular SHA-256, MIME, tamanho e dimensoes do PNG.
6. Solicitar ao bunker uma assinatura para o evento de autorizacao Blossom
   BUD-11 (`kind 24242`) e enviar `PUT /upload` ao servidor configurado.
7. Validar o Blob Descriptor retornado: URL HTTPS, SHA-256, tamanho e MIME
   devem corresponder ao arquivo enviado.
8. Montar uma nota Nostr `kind 1` com:
   - URL retornada pelo Blossom no conteudo.
   - Comando de reproducao em texto simples.
   - Link do repositorio GitHub.
   - Tag `imeta` NIP-92 com `url`, `m`, `x`, `size`, `dim` e `alt`.
9. Assinar a nota pelo bunker e publicá-la em cada relay configurado via `nak`.
10. Exigir confirmacao `OK` de pelo menos um relay. Exibir URL do Blob e ID do
    evento em `stderr` apenas depois do sucesso.
11. Mover PNG e JSON para `FRACT_OUTPUT_DIR` usando um nome sequencial ou
    manter os artefatos temporarios somente com `--keep-failed`.

## Uso de Nak e Bunker

O script usa `nak blossom --server HOST --sec BUNKER upload ARQUIVO`, que cria
a autorizacao Blossom pelo bunker, e `nak event --sec BUNKER ... RELAY...` para
assinar e publicar a nota. `NOSTR_CLIENT_KEY` recebe uma chave efemera criada
por `nak key generate` para a sessao NIP-46 e e removida no encerramento.

Nao ha fallback para chave privada local. Se o bunker ou o servidor Blossom
recusar a operacao, o script falha sem publicar a nota.

## Conteudo Padrao da Nota

```text
Imagem gerada com fract.

<URL_BLOSSOM>

Reproduzir:
fract render mandelbrot ...

Projeto:
https://github.com/girino/random-mandelbrot-generator
```

`--content` antecede esse bloco. O texto alternativo padrao sera: `Imagem do
conjunto de Mandelbrot gerada proceduralmente.`

## Falhas e Seguranca

- Nunca imprimir URI do bunker, segredo de sessao ou cabecalhos Authorization.
- Interromper se o upload retornar dados inconsistentes. Algumas versoes do
  `nak` podem imprimir um Blob Descriptor incompleto, embora o upload tenha
  sido aceito pelo servidor; nesse caso, validar o SHA-256 no nome da URL e
  usar tamanho e MIME locais com aviso. Com `--keep-failed`, preservar tambem
  `blob-descriptor.json` para diagnostico.
- Nao publicar se a assinatura, upload ou todos os relays falharem.
- Nao apagar PNG ou metadados em uma falha; preserva-los quando solicitado com
  `--keep-failed`.
- Usar arquivos temporarios com permissoes restritas e limpa-los em `trap`.
- `--dry-run` nao deve solicitar assinatura, enviar bytes nem conectar a relays.

## Validacao

1. Executar `bash scripts/test-publish-nostr.sh` para validar sintaxe e
   composicao estatica.
2. Testar `--dry-run` com `fract` e `jq` instalados.
3. Validar que o evento contem URL da imagem, `imeta`, comando e link GitHub.
4. Validar falha quando o SHA-256 ou MIME do Blob Descriptor divergir.
5. Validar sucesso parcial com um relay aceitando e outro falhando.
6. Testar em Linux, Git Bash e WSL com caminhos locais apropriados.
