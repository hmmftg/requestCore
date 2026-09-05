// Package main demonstrates a checkout saga with three steps:
// reserve inventory, charge card, and ship order. If any step fails,
// completed steps are compensated in reverse order.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hmmftg/requestCore/v2/saga"
)

func main() {
	store := saga.NewMemoryStore()

	orch := saga.NewOrchestrator(
		saga.WithStore(store),
		saga.WithTimeout(10*time.Second),
	)

	s := &saga.Saga{
		ID:   "checkout-" + fmt.Sprint(time.Now().UnixMilli()),
		Name: "checkout",
		Steps: []saga.Step{
			{
				Name: "reserve-inventory",
				Execute: func(_ context.Context, st *saga.SagaState) error {
					st.SetData("inventoryReserved", true)
					fmt.Println("  -> Reserved inventory")
					st.AppendOutbox(saga.OutboxRecord{
						ID:        st.ID + "-inv-event",
						SagaID:    st.ID,
						StepName:  "reserve-inventory",
						EventType: "InventoryReserved",
						Payload:   []byte(`{"saga":"` + st.ID + `"}`),
						CreatedAt: time.Now(),
					})
					return nil
				},
				Compensate: func(_ context.Context, _ *saga.SagaState) error {
					fmt.Println("  <- Released inventory")
					return nil
				},
			},
			{
				Name: "charge-card",
				Execute: func(_ context.Context, st *saga.SagaState) error {
					st.SetData("cardCharged", true)
					fmt.Println("  -> Charged card")
					return nil
				},
				Compensate: func(_ context.Context, _ *saga.SagaState) error {
					fmt.Println("  <- Refunded card")
					return nil
				},
			},
			{
				Name: "ship-order",
				Execute: func(_ context.Context, _ *saga.SagaState) error {
					fmt.Println("  -> Shipped order")
					return nil
				},
			},
		},
	}

	fmt.Println("=== Happy Path ===")
	if err := orch.Execute(context.Background(), s); err != nil {
		log.Fatalf("happy path failed: %v", err)
	}
	fmt.Println("Saga completed successfully!\n")

	failingSaga := &saga.Saga{
		ID:   "checkout-fail-" + fmt.Sprint(time.Now().UnixMilli()),
		Name: "checkout",
		Steps: []saga.Step{
			{
				Name: "reserve-inventory",
				Execute: func(_ context.Context, _ *saga.SagaState) error {
					fmt.Println("  -> Reserved inventory")
					return nil
				},
				Compensate: func(_ context.Context, _ *saga.SagaState) error {
					fmt.Println("  <- Released inventory")
					return nil
				},
			},
			{
				Name: "charge-card",
				Execute: func(_ context.Context, _ *saga.SagaState) error {
					fmt.Println("  -> Charged card")
					return nil
				},
				Compensate: func(_ context.Context, _ *saga.SagaState) error {
					fmt.Println("  <- Refunded card")
					return nil
				},
			},
			{
				Name: "ship-order",
				Execute: func(_ context.Context, _ *saga.SagaState) error {
					fmt.Println("  -> Ship order: carrier unavailable!")
					return fmt.Errorf("carrier unavailable")
				},
				Compensate: func(_ context.Context, _ *saga.SagaState) error {
					fmt.Println("  <- Cancelled shipment")
					return nil
				},
			},
		},
	}

	fmt.Println("=== Failure Path (step 3 fails) ===")
	err := orch.Execute(context.Background(), failingSaga)
	if err != nil {
		fmt.Printf("Saga failed as expected: %v\n", err)
	}
	fmt.Println("Compensation completed!\n")

	st, _ := store.Load(context.Background(), failingSaga.ID)
	fmt.Printf("Final saga status: %s\n", st.Status)
	for i, step := range st.Steps {
		fmt.Printf("  Step %d (%s): %s\n", i, step.Name, step.Status)
	}
}
