// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package command

import (
	"fmt"
	"io/ioutil"

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/dm/ansible"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster"
	cansible "github.com/pingcap/tiup/pkg/cluster/ansible"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func newImportCmd() *cobra.Command {
	var ansibleDir string
	var inventoryFileName string
	var rename string
	var clusterVersion string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import an exist DM 1.0 cluster from dm-ansible and re-deploy 2.0 version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := supportVersion(clusterVersion); err != nil {
				return errors.AddStack(err)
			}

			importer, err := ansible.NewImporter(ansibleDir, inventoryFileName, gOpt.NativeSSH, gOpt.SSHTimeout)
			if err != nil {
				return errors.AddStack(err)
			}

			clusterName, meta, err := importer.ImportFromAnsibleDir()
			if err != nil {
				return errors.AddStack(err)
			}

			if rename != "" {
				clusterName = rename
			}

			err = importer.ScpSourceToMaster(meta.Topology)
			if err != nil {
				return errors.AddStack(err)
			}

			data, err := yaml.Marshal(meta.Topology)
			if err != nil {
				return errors.AddStack(err)
			}

			f, err := ioutil.TempFile("", "tiup-*")
			if err != nil {
				return errors.AddStack(err)
			}

			_, err = f.Write(data)
			if err != nil {
				return errors.AddStack(err)
			}

			fmt.Println(color.HiYellowString("Will use the following topology to deploy a DM cluster: "))
			fmt.Println(string(data))

			if !skipConfirm {
				err = cliutil.PromptForConfirmOrAbortError(
					"Using the Topology to deploy DM %s cluster %s, Do you want to continue? [y/N]: ",
					clusterVersion,
					clusterName,
				)
				if err != nil {
					return errors.AddStack(err)
				}
			}

			err = manager.Deploy(
				clusterName,
				clusterVersion,
				f.Name(),
				cluster.DeployOptions{
					IdentityFile: cansible.SSHKeyPath(),
					User:         tiuputils.CurrentUser(),
				},
				nil,
				skipConfirm,
				gOpt.OptTimeout,
				gOpt.SSHTimeout,
				gOpt.NativeSSH,
			)

			if err != nil {
				return errors.AddStack(err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&ansibleDir, "dir", "d", "./", "The path to DM-Ansible directory")
	cmd.Flags().StringVar(&inventoryFileName, "inventory", cansible.AnsibleInventoryFile, "The name of inventory file")
	cmd.Flags().StringVarP(&rename, "rename", "r", "", "Rename the imported cluster to `NAME`")
	cmd.Flags().StringVarP(&clusterVersion, "cluster-version", "v", "nightly", "cluster version of DM")

	return cmd
}