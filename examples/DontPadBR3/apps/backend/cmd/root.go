package cmd

import (
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/app"
	"github.com/spf13/cobra"
)

// NewRootCommand builds the DontPad backend command tree.
func NewRootCommand() *cobra.Command {
	opts := app.Options{EnvFile: ".env"}

	rootCmd := &cobra.Command{
		Use:          "dontpad-backend",
		Short:        "DontPad backend server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.RunServer(opts)
		},
	}

	rootCmd.PersistentFlags().StringVar(&opts.EnvFile, "env-file", opts.EnvFile, "arquivo .env para carregar antes das variáveis de ambiente")
	rootCmd.PersistentFlags().StringVar(&opts.DatabaseURL, "database-url", "", "Postgres DSN; sobrescreve DATABASE_URL")
	rootCmd.PersistentFlags().StringVar(&opts.Address, "addr", "", "endereço HTTP do backend; sobrescreve DONTPAD_ADDR")
	rootCmd.PersistentFlags().StringVar(&opts.Schema, "schema", "", "schema Postgres; sobrescreve DONTPAD_SCHEMA")
	rootCmd.PersistentFlags().StringVar(&opts.Namespace, "namespace", "", "namespace Yjs; sobrescreve DONTPAD_NAMESPACE")
	rootCmd.PersistentFlags().StringVar(&opts.DataDir, "data-dir", "", "diretório de storage; sobrescreve DONTPAD_DATA_DIR")
	rootCmd.PersistentFlags().StringVar(&opts.AllowedOrigins, "allowed-origins", "", "origins CORS/WebSocket separados por vírgula; sobrescreve DONTPAD_ALLOWED_ORIGINS")
	rootCmd.PersistentFlags().StringVar(&opts.AuthSecret, "jwt-secret", "", "segredo JWT/HMAC; sobrescreve JWT_SECRET")
	rootCmd.PersistentFlags().StringVar(&opts.MasterPassword, "master-password", "", "senha mestre opcional; sobrescreve MASTER_PASSWORD")

	serveCmd := &cobra.Command{
		Use:          "serve",
		Short:        "Inicia o backend HTTP/WebSocket",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.RunServer(opts)
		},
	}
	rootCmd.AddCommand(serveCmd)

	return rootCmd
}
