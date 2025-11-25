from openai import OpenAI

client = OpenAI(
    api_key="demo-key-12345",  # Demo API key from seed data
    base_url="http://localhost:8080/v1",
)

resp = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "What's the capital of France?"}]
)

print(resp.choices[0].message.content)
