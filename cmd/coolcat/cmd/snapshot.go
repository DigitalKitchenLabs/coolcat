package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/spf13/cobra"
)

type CoolCatSnapshot struct {
	TotalCatdropAmount sdk.Int                    `json:"total_catdrop_amount"`
	Accounts                map[string]CoolCatSnapshotAccount `json:"accounts"`
}

type CoolCatSnapshotAccount struct {
	AtomAddress              string  `json:"atom_address"`
	JunoAddress              string  `json:"juno_address"`
	HuahuaAddress            string  `json:"huahua_address"`
	OutsideTopTwenty  		 bool    `json:"atom_bonus"`
	AtomStaker               bool    `json:"atom_staker"`
	JunoStaker               bool    `json:"juno_staker"`
	HuahuaStaker             bool    `json:"huahua_staker"`
	AirdropAmount            sdk.Int `json:"airdrop_amount"`
}

func GenerateSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-snapshot [input-hub-snapshot] [input-juno-snapshot] [input-huahua-snapshot] [output-snapshot]",
		Short: "Generate final snapshot from a provided snapshots",
		Long: `Generate final snapshot from a provided snapshots
Example:
	coolcatd generate-snapshot hub-snapshot.json juno-snapshot.json huahua-snapshot.json snapshot.json
`,
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config

			config.SetRoot(clientCtx.HomeDir)

			hubSnapshotFile := args[0]
			junoSnapshotFile := args[1]
			huahuaSnapshotFile := args[2]
			snapshotOutput := args[3]

			hubJSON, _ := os.ReadFile(hubSnapshotFile)
			junoJSON, _ := os.ReadFile(junoSnapshotFile)
			huahuaJSON, _ := os.ReadFile(huahuaSnapshotFile)

			snapshotAccs := make(map[string]CoolCatSnapshotAccount)

			hubSnapshot := HubSnapshot{}
			json.Unmarshal([]byte(hubJSON), &hubSnapshot)
			for _, staker := range hubSnapshot.Accounts {
				coolcatAddr, err := ConvertAddressToCoolCat(staker.AtomAddress)
				if err != nil {
					panic(err)
				}
				// create account for the first time
				snapshotAcc := CoolCatSnapshotAccount{
					AtomAddress:	         staker.AtomAddress,
					JunoAddress:             "",
					HuahuaAddress:           "",
					OutsideTopTwenty:  		 staker.OutsideTopTwenty,
					AtomStaker:              staker.AtomStaker,
					JunoStaker:              false,
					HuahuaStaker:            false,
				}
				snapshotAccs[coolcatAddr.String()] = snapshotAcc
			}

			junoSnapshot := JunoSnapshot{}
			json.Unmarshal([]byte(junoJSON), &junoSnapshot)
			for _, acct := range junoSnapshot.Accounts {

				coolcatAddr, err := ConvertAddressToCoolCat(acct.JunoAddress)
				if err != nil {
					panic(err)
				}
				if acc, ok := snapshotAccs[coolcatAddr.String()]; ok {
					// account exists
					acc.JunoAddress = acct.JunoAddress
					acc.JunoStaker = acct.JunoStaker
					snapshotAccs[coolcatAddr.String()] = acc
				} else {
					// account does not exist, create it
					snapshotAcc := CoolCatSnapshotAccount{
						JunoAddress:              acct.JunoAddress,
						JunoStaker:               acct.JunoStaker,
					}
					snapshotAccs[coolcatAddr.String()] = snapshotAcc
				}
			}

			huahuaSnapshot := HuahuaSnapshot{}
			json.Unmarshal([]byte(huahuaJSON), &huahuaSnapshot)
			for _, acct := range huahuaSnapshot.Accounts {
				coolcatAddr, err := ConvertAddressToCoolCat(acct.HuahuaAddress)
				if err != nil {
					panic(err)
				}
				if acc, ok := snapshotAccs[coolcatAddr.String()]; ok {
					// account exists
					acc.HuahuaAddress = acct.HuahuaAddress
					acc.HuahuaStaker = acct.HuahuaStaker
					snapshotAccs[coolcatAddr.String()] = acc
				} else {
					// account does not exist, create it
					snapshotAcc := CoolCatSnapshotAccount{
						HuahuaAddress:           acct.HuahuaAddress,
						HuahuaStaker: acct.HuahuaStaker,
					}
					snapshotAccs[coolcatAddr.String()] = snapshotAcc
				}
			}

			// calculate number of rewards
			numRewards := 0
			numAtomBonusRewards := 0
			for _, acct := range snapshotAccs {
				if acct.JunoStaker {
					numRewards++
				}
				if acct.HuahuaStaker {
					numRewards++
				}
				if acct.AtomStaker {
					if acct.OutsideTopTwenty {
						numAtomBonusRewards++
					}
					numRewards++
				}

			}

			airdropSupply := sdk.NewInt(3_500_000_000_000_000) // 3.500.000.000 CCAT in uccat (1CCAT = 1e-6 uucat)
			baseReward := airdropSupply.QuoRaw(int64(numRewards + numAtomBonusRewards)) // 49,472,761,710,909 ~= 49,472 UCCAT per reward

			// calculate airdrop amount
			for addr, acct := range snapshotAccs {
				amt := sdk.ZeroInt()
				if acct.AtomStaker {
					amt = amt.Add(baseReward)
					if acct.OutsideTopTwenty {
						amt = amt.Add(baseReward)
					}
				}
				if acct.HuahuaStaker {
					amt = amt.Add(baseReward)
				}
				if acct.JunoStaker {
					amt = amt.Add(baseReward)
				}
				acct.AirdropAmount = amt
				snapshotAccs[addr] = acct
			}

			average := airdropSupply.QuoRaw(int64(len(snapshotAccs))) // 51,862,608,541,030

			snapshot := CoolCatSnapshot{
				TotalCatdropAmount: 	airdropSupply,
				Accounts:                snapshotAccs,
			}

			fmt.Println("=== CoolCat Catdrop Generator ===")
			fmt.Printf("👥 Total Accounts: %d\n", len(snapshotAccs))
			fmt.Println("---------")
			fmt.Printf("🔐 Staking Rewards: %d\n", numRewards)
			fmt.Printf("🔝 Outside-Top20 Staking Rewards: %d\n", numAtomBonusRewards)
			fmt.Println("---------")
			fmt.Printf("✨ Reward Amount: %.2f $CCAT\n", float64(math.Floor(sdk.NewDecWithPrec(baseReward.Int64(), 6).Mul(sdk.NewDec(int64(100))).MustFloat64()) / 100))
			fmt.Printf("✅ Average Reward Amount: %.2f $CCAT\n", float64(math.Floor(sdk.NewDecWithPrec(average.Int64(), 6).Mul(sdk.NewDec(int64(100))).MustFloat64()) / 100))

			// export snapshot json
			snapshotJSON, err := json.MarshalIndent(snapshot, "", "    ")
			if err != nil {
				return fmt.Errorf("failed to marshal snapshot: %w", err)
			}

			err = os.WriteFile(snapshotOutput, snapshotJSON, 0600)
			return err
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func ConvertAddressToCoolCat(address string) (sdk.AccAddress, error) {
	config := sdk.GetConfig()
	ccatPrefix := config.GetBech32AccountAddrPrefix()

	_, bytes, err := bech32.DecodeAndConvert(address)
	if err != nil {
		return nil, err
	}

	newAddr, err := bech32.ConvertAndEncode(ccatPrefix, bytes)
	if err != nil {
		return nil, err
	}

	sdkAddr, err := sdk.AccAddressFromBech32(newAddr)
	if err != nil {
		return nil, err
	}

	return sdkAddr, nil
}