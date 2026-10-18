"""Evaluate a PEFT reference using the same text fixtures as the deployed package."""
import argparse
import json
from pathlib import Path
import time
import sys
sys.path.insert(0, str(Path(__file__).resolve().parent))
from train import sha


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("source", type=Path)
    parser.add_argument("adapter", type=Path)
    parser.add_argument("bot", type=Path)
    parser.add_argument("fixtures", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    import torch
    from transformers import AutoModelForCausalLM, AutoTokenizer, BitsAndBytesConfig, set_seed
    from peft import PeftModel
    set_seed(42)
    bot = json.loads(args.bot.read_text())
    tokenizer = AutoTokenizer.from_pretrained(args.source, local_files_only=True)
    model = AutoModelForCausalLM.from_pretrained(args.source, local_files_only=True,
        quantization_config=BitsAndBytesConfig(load_in_4bit=True, bnb_4bit_quant_type="nf4",
        bnb_4bit_use_double_quant=True, bnb_4bit_compute_dtype=torch.bfloat16), device_map={"": 0})
    model = PeftModel.from_pretrained(model, args.adapter, local_files_only=True)
    model.eval()
    cases = []
    for fixture in json.loads(args.fixtures.read_text()):
        if fixture.get("min_tool_calls", 0):
            raise ValueError("Reference text equivalence suite cannot contain executable tool cases")
        messages = [{"role": "system", "content": bot["instructions"]},
                    {"role": "user", "content": fixture["input"]}]
        inputs = tokenizer.apply_chat_template(messages, add_generation_prompt=True,
            enable_thinking=False, return_tensors="pt").to("cuda")
        started = time.time()
        with torch.no_grad():
            generated = model.generate(inputs, attention_mask=torch.ones_like(inputs), do_sample=False,
                max_new_tokens=bot["generation"]["max_tokens"], pad_token_id=tokenizer.eos_token_id)
        output = tokenizer.decode(generated[0, inputs.shape[1]:], skip_special_tokens=True).strip()
        passed = all(term.lower() in output.lower() for term in fixture.get("contains", []))
        if "expected_json" in fixture:
            try:
                passed = passed and json.loads(output) == fixture["expected_json"]
            except ValueError:
                passed = False
        cases.append({"fixture": fixture["name"], "passed": passed, "output": output,
            "seconds": time.time() - started, "prompt_tokens": inputs.shape[1],
            "completion_tokens": generated.shape[1] - inputs.shape[1]})
    evidence = {"cases": cases, "pass_rate": sum(c["passed"] for c in cases) / len(cases),
        "fixtures_sha256": sha(args.fixtures), "adapter_sha256": sha(args.adapter / "adapter_model.safetensors"),
        "base": json.loads((args.adapter / "training.json").read_text())["base"],
        "reference_quantization": "NF4 double", "deployed_quantization": "Q8_0",
        "note": "Compare task correctness and outputs; numerical identity is not expected across quantizations."}
    args.output.write_text(json.dumps(evidence, indent=2))


if __name__ == "__main__":
    main()
