package cmd

import (
	"fmt"
	"github.com/jsiebens/hashi-up/pkg/config"
	"github.com/jsiebens/hashi-up/pkg/operator"
	"github.com/markbates/pkger"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/thanhpk/randstr"
	"path/filepath"
)

func InstallBoundaryCommand() *cobra.Command {

	var skipEnable bool
	var skipStart bool
	var binary string
	var version string

	var initDatabase bool
	var configFile string
	var files []string

	var command = &cobra.Command{
		Use:          "install",
		SilenceUsage: true,
	}

	command.Flags().BoolVar(&skipEnable, "skip-enable", false, "If set to true will not enable or start Boundary service")
	command.Flags().BoolVar(&skipStart, "skip-start", false, "If set to true will not start Boundary service")
	command.Flags().StringVarP(&binary, "package", "p", "", "Upload and use this Boundary package instead of downloading")
	command.Flags().StringVarP(&version, "version", "v", "", "Version of Boundary to install")

	command.Flags().BoolVarP(&initDatabase, "init-database", "d", false, "Initialize the Boundary database")
	command.Flags().StringVarP(&configFile, "config-file", "c", "", "Custom Consul configuration file to upload")
	command.Flags().StringArrayVarP(&files, "file", "f", []string{}, "Additional files, e.g. certificates, to upload")

	command.RunE = func(command *cobra.Command, args []string) error {
		if !runLocal && len(sshTargetAddr) == 0 {
			return fmt.Errorf("required ssh-target-addr flag is missing")
		}

		if len(binary) == 0 && len(version) == 0 {
			latest, err := config.GetLatestVersion("boundary")

			if err != nil {
				return errors.Wrapf(err, "unable to get latest version number, define a version manually with the --version flag")
			}

			version = latest
		}

		callback := func(op operator.CommandOperator) error {
			dir := "/tmp/boundary-installation." + randstr.String(6)

			defer op.Execute("rm -rf " + dir)

			_, err := op.Execute("mkdir -p " + dir + "/config")
			if err != nil {
				return fmt.Errorf("error received during installation: %s", err)
			}

			if len(binary) != 0 {
				info("Uploading Boundary package...")
				err = op.UploadFile(binary, dir+"/boundary.zip", "0640")
				if err != nil {
					return fmt.Errorf("error received during upload Boundary package: %s", err)
				}
			}

			info(fmt.Sprintf("Uploading %s as boundary.hcl...", configFile))
			err = op.UploadFile(expandPath(configFile), dir+"/config/boundary.hcl", "0640")
			if err != nil {
				return fmt.Errorf("error received during upload boundary configuration: %s", err)
			}

			for _, s := range files {
				info(fmt.Sprintf("Uploading %s...", s))
				_, filename := filepath.Split(expandPath(s))
				err = op.UploadFile(expandPath(s), dir+"/config/"+filename, "0640")
				if err != nil {
					return fmt.Errorf("error received during upload file: %s", err)
				}
			}

			installScript, err := pkger.Open("/scripts/install_boundary.sh")

			if err != nil {
				return err
			}

			defer installScript.Close()

			err = op.Upload(installScript, dir+"/install.sh", "0755")
			if err != nil {
				return fmt.Errorf("error received during upload install script: %s", err)
			}

			info("Installing Boundary...")
			_, err = op.Execute(fmt.Sprintf("cat %s/install.sh | TMP_DIR='%s' INIT_DATABASE='%t' BOUNDARY_VERSION='%s' SKIP_ENABLE='%t' SKIP_START='%t' sh -\n", dir, dir, initDatabase, version, skipEnable, skipStart))
			if err != nil {
				return fmt.Errorf("error received during installation: %s", err)
			}

			return nil
		}

		if runLocal {
			return operator.ExecuteLocal(callback)
		} else {
			return operator.ExecuteRemote(sshTargetAddr, sshTargetUser, sshTargetKey, callback)
		}
	}

	return command
}
