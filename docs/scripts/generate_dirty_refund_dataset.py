from pathlib import Path
import pandas as pd


input_path = Path("data/clean_refund_dataset.csv")
output_path = Path("data/dirty_business_refund_dataset.csv")

df = pd.read_csv(input_path)


# Clean internal columns -> dirty business columns
column_mapping = {
    "order_id": "purchase_id",
    "customer_id": "buyer_id",
    "return_id": "refund_request_id",
    "support_agent_id": "agent_id",
    "order_amount": "purchase_amount",
    "refund_amount": "return_amount",
    "product_category": "category",
    "return_reason": "reason",
    "evidence_provided": "has_photo",
    "decision": "status",
    "manual_override": "override",
    "decision_time_minutes": "resolution_minutes",
    "timestamp": "created_at",
}

dirty_df = df.rename(columns=column_mapping)


# Make IDs look more like raw business system IDs
dirty_df["purchase_id"] = dirty_df["purchase_id"].str.replace("order_", "purchase_", regex=False)
dirty_df["buyer_id"] = dirty_df["buyer_id"].str.replace("customer_", "buyer_", regex=False)
dirty_df["refund_request_id"] = dirty_df["refund_request_id"].str.replace("return_", "refund_req_", regex=False)


# Make boolean fields look like business CSV values
dirty_df["has_photo"] = dirty_df["has_photo"].map({
    True: "yes",
    False: "no",
    "True": "yes",
    "False": "no",
})

dirty_df["override"] = dirty_df["override"].map({
    True: "yes",
    False: "no",
    "True": "yes",
    "False": "no",
})


# Make status look less internal
dirty_df["status"] = dirty_df["status"].str.lower()


# Convert timestamp from ISO UTC style to business export style
dirty_df["created_at"] = (
    pd.to_datetime(dirty_df["created_at"])
    .dt.strftime("%Y-%m-%d %H:%M:%S")
)


dirty_df.to_csv(output_path, index=False)

print(f"Created dirty business dataset: {output_path}")
print(f"Rows: {len(dirty_df)}")
print(f"Columns: {list(dirty_df.columns)}")