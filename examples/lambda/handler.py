# handler.py — hello lambda example
def handler(event, context=None):
    name = (event or {}).get("name", "world") if isinstance(event, dict) else "world"
    return {"message": f"Hello, {name}!"}


if __name__ == "__main__":
    print(handler({"name": "tinyaws"}))
