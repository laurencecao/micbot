import torch


def main():
    model_dir = "FunAudioLLM/Fun-ASR-Nano-2512"
    device = (
        "cuda:0"
        if torch.cuda.is_available()
        else "mps"
        if torch.backends.mps.is_available()
        else "cpu"
    )

    from funasr import AutoModel

    #wav_path = f"{model.model_path}/example/zh.mp3"
    wav_path = '/home/ubuntu/mp3/mp3/飞书20260109-111737.mp3'
    wav_path = '/home/ubuntu/mp3/mp3/1768197492078_0112会议纪要.mp3'
    wav_path = '/home/ubuntu/myasr2/recording_20260118_113524.wav'

    model = AutoModel(
        model=model_dir,
        trust_remote_code=True,
        vad_model="fsmn-vad",
        vad_kwargs={"max_single_segment_time": 30000},
        remote_code="./model.py",
        device=device,
    )
    res = model.generate(input=[wav_path], cache={}, 
            use_itn=True,
            batch_size=1,
            batch_size_s=0,
            merge_vad=True, 
            merge_length_s=15,
            ban_emo_unk=True,
            output_timestamp=True)
    #res = model.generate(input=[wav_path], cache={}, batch_size=1, use_itn=False, batch_size_s=0, )
    text = res[0]["text"]
    print(text)
    with open("/tmp/r2.txt", 'w') as wfl:
        wfl.write(text)


if __name__ == "__main__":
    main()
