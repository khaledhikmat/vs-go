
import torch
from models.common import DetectMultiBackend
from pathlib import Path

def export_yolov5_to_onnx(weights_path='yolov5s.pt', img_size=640, batch_size=1, onnx_path='yolov5s.onnx'):
    device = torch.device('cpu')
    model = DetectMultiBackend(weights_path, device=device)
    model.eval()

    # Dummy input for export
    dummy_input = torch.zeros(batch_size, 3, img_size, img_size).to(device)

    # Export to ONNX
    torch.onnx.export(
        model.model,
        dummy_input,
        onnx_path,
        verbose=False,
        opset_version=12,
        input_names=['images'],
        output_names=['output'],
        dynamic_axes={'images': {0: 'batch'}, 'output': {0: 'batch'}}
    )
    print(f"✅ Exported to {onnx_path}")

if __name__ == '__main__':
    export_yolov5_to_onnx()
