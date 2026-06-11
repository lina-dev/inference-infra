# Custom Triton image with the Whisper ASR model repository baked in, so every
# new Triton pod loads the model on startup with no external/EFS model store.
#
# Build context is this directory (models/asr):
#   docker build -t inference-infra/triton-whisper:latest models/asr
#
# Repository layout produced inside the image (Triton loads it from /models):
#   /models/whisper/config.pbtxt
#   /models/whisper/1/model.py
ARG TRITON_VERSION=24.01
FROM nvcr.io/nvidia/tritonserver:${TRITON_VERSION}-py3

# Python backend dependencies for models/asr/model.py (imports `whisper`).
RUN pip install --no-cache-dir openai-whisper

# Bake the Triton model repository.
COPY config.pbtxt /models/whisper/config.pbtxt
COPY model.py     /models/whisper/1/model.py

# Pre-download the Whisper weights into the image so pods start fully offline
# and skip a ~3 GB per-pod download. Disable for a much smaller image with:
#   docker build --build-arg BAKE_WEIGHTS=false ...
# (model.py loads "large-v3"; keep WHISPER_MODEL in sync if you change it.)
ARG BAKE_WEIGHTS=true
ARG WHISPER_MODEL=large-v3
RUN if [ "$BAKE_WEIGHTS" = "true" ]; then \
        python3 -c "import whisper; whisper.load_model('${WHISPER_MODEL}')"; \
    fi

# The Deployment supplies the `tritonserver ...` command via args, matching the
# base image's entrypoint wrapper — so no ENTRYPOINT override here.
