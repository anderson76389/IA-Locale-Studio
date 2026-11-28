package fr.ialocale.mobile;

import android.graphics.*;
import org.json.*;
import org.jcodec.api.android.AndroidSequenceEncoder;
import org.jcodec.common.io.NIOUtils;
import org.jcodec.common.io.SeekableByteChannel;
import org.jcodec.common.model.Rational;
import java.io.*;
import java.nio.*;
import java.nio.charset.StandardCharsets;
import java.util.function.BooleanSupplier;

final class MediaWorkshop {
    static String extension(String kind) {
        switch(kind){case "Image":return "png";case "Vidéo":return "mp4";case "Musique":return "wav";default:throw new IllegalArgumentException(kind);}
    }
    static String instructions(String kind) {
        if(kind.equals("Musique"))
            return "Compose an instrumental melody matching the user's request. Output JSON [tempo,voice,[note,...]]. Tempo is beats per minute, voice 0 soft keys, 1 pluck, 2 flute. Each note is [MIDI pitch, duration in quarter-beat units]. Vary melody and rhythm. Request: ";
        return "Design an original layered flat vector illustration matching the request. Output JSON [background hex color,[shape,...]]. Each shape is [type,hex color,x,y,width,height,endX,endY]. Type 0 ellipse, 1 rectangle, 2 triangle, 3 line. Coordinates and sizes are percentages (0-99); x/y is top left. End coordinates animate the shape in a video. Compose foreground and background with contrasting colors. Request: ";
    }
    static String grammar(String kind) {
        String number="n ::= [0-9] | [1-9] [0-9]\n";
        if(kind.equals("Musique"))
            return "root ::= \"[\" bpm \",\" [0-2] \",[\" note (\",\" note){7,23} \"]]\"\nbpm ::= [6-9] [0-9] | \"1\" [0-5] [0-9]\nnote ::= \"[\" [4-8] [0-9] \",\" [1-8] \"]\"\n";
        return "root ::= \"[\" color \",[\" shape (\",\" shape){2,9} \"]]\"\ncolor ::= \"\\\"#\" [0-9A-Fa-f]{6} \"\\\"\"\nshape ::= \"[\" [0-3] \",\" color (\",\" n){6} \"]\"\n"+number;
    }
    static File render(File folder,String kind,String spec,BooleanSupplier cancelled) throws Exception {
        if(!folder.mkdirs()&&!folder.isDirectory())throw new IOException("Dossier de création inaccessible");
        File file=new File(folder,"creation."+extension(kind));
        JSONArray plan=new JSONArray(spec);
        boolean done=false;
        try{
            if(kind.equals("Musique"))writeMusic(file,plan,cancelled);
            else{
                boolean video=kind.equals("Vidéo");
                Bitmap bitmap=Bitmap.createBitmap(video?384:768,video?384:768,Bitmap.Config.ARGB_8888);
                try{
                    if(video){
                        try(SeekableByteChannel channel=NIOUtils.writableChannel(file)){
                            AndroidSequenceEncoder enc=new AndroidSequenceEncoder(channel,Rational.R(12,1));
                            for(int frame=0;frame<36;frame++){
                                check(cancelled);draw(bitmap,plan,frame/35f);enc.encodeImage(bitmap);
                            }
                            enc.finish();
                        }
                    }else try(FileOutputStream stream=new FileOutputStream(file)){
                        draw(bitmap,plan,0);if(!bitmap.compress(Bitmap.CompressFormat.PNG,100,stream))throw new IOException("Encodage PNG impossible");
                    }
                }finally{bitmap.recycle();}
            }
            check(cancelled);done=true;return file;
        }finally{if(!done){file.delete();folder.delete();}}
    }
    private static void check(BooleanSupplier cancelled)throws IOException{if(cancelled.getAsBoolean())throw new IOException("Création annulée");}
    private static void draw(Bitmap bitmap,JSONArray plan,float time)throws JSONException{
        Canvas canvas=new Canvas(bitmap);canvas.drawColor(Color.parseColor(plan.getString(0)));
        Paint brush=new Paint(Paint.ANTI_ALIAS_FLAG);float scale=bitmap.getWidth()/100f;
        canvas.scale(scale,scale);
        JSONArray layers=plan.getJSONArray(1);
        for(int i=0;i<layers.length();i++){
            JSONArray shape=layers.getJSONArray(i);brush.setColor(Color.parseColor(shape.getString(1)));
            float x=(float)(shape.getDouble(2)*(1-time)+shape.getDouble(6)*time);
            float y=(float)(shape.getDouble(3)*(1-time)+shape.getDouble(7)*time);
            float w=Math.max(1,shape.getInt(4)),h=Math.max(1,shape.getInt(5));
            switch(shape.getInt(0)){
                case 0:canvas.drawOval(x,y,x+w,y+h,brush);break;
                case 1:canvas.drawRect(x,y,x+w,y+h,brush);break;
                case 2:Path p=new Path();p.moveTo(x+w/2,y);p.lineTo(x+w,y+h);p.lineTo(x,y+h);p.close();canvas.drawPath(p,brush);break;
                case 3:brush.setStrokeWidth(1.5f);canvas.drawLine(x,y,x+w,y+h,brush);break;
                default:throw new JSONException("Forme inconnue");
            }
        }
    }
    private static void writeMusic(File file,JSONArray plan,BooleanSupplier cancelled)throws Exception{
        int rate=22050,tempo=plan.getInt(0),voice=plan.getInt(1),samples=0;
        JSONArray notes=plan.getJSONArray(2);
        int unit=rate*60/(4*tempo);
        for(int i=0;i<notes.length();i++)samples+=unit*notes.getJSONArray(i).getInt(1);
        ByteBuffer header=ByteBuffer.allocate(44).order(ByteOrder.LITTLE_ENDIAN);
        header.put("RIFF".getBytes(StandardCharsets.US_ASCII)).putInt(36+samples*2).put("WAVEfmt ".getBytes(StandardCharsets.US_ASCII))
            .putInt(16).putShort((short)1).putShort((short)1).putInt(rate).putInt(rate*2).putShort((short)2).putShort((short)16)
            .put("data".getBytes(StandardCharsets.US_ASCII)).putInt(samples*2);
        try(OutputStream out=new BufferedOutputStream(new FileOutputStream(file))){
            out.write(header.array());
            for(int i=0;i<notes.length();i++){
                check(cancelled);JSONArray note=notes.getJSONArray(i);int length=unit*note.getInt(1);
                double hz=440*Math.pow(2,(note.getInt(0)-69)/12.0);
                for(int j=0;j<length;j++){
                    double t=j/(double)rate,phase=t*2*Math.PI*hz;
                    double env=Math.min(1,t*80)*Math.min(1,(length-j)/(rate*.045))*Math.exp(-t*(voice==1?5:1.2));
                    double wave=Math.sin(phase)+(.14+.08*voice)*Math.sin(phase*2)+.08*Math.sin(phase*3);
                    int sample=(int)(env*wave*18000);out.write(sample);out.write(sample>>8);
                }
            }
        }
    }
}
