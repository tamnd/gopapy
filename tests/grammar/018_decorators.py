@app.route("/")
def index():
    return "ok"

@trace
@cache(ttl=60)
class Service:
    pass
