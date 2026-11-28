package fr.ialocale.mobile;

import android.content.Context;
import android.media.*;
import android.net.Uri;
import com.google.android.gms.tasks.Tasks;
import com.google.mlkit.vision.common.InputImage;
import com.google.mlkit.vision.text.*;
import com.google.mlkit.vision.text.latin.TextRecognizerOptions;
import org.json.JSONObject;
import org.vosk.Model;
import org.vosk.Recognizer;
import java.io.*;
import java.nio.*;
import java.nio.file.*;
import java.util.zip.*;
import java.util.function.BooleanSupplier;

final class Attachments {
    static String image(Context ctx,Uri uri) throws Exception {
        try(TextRecognizer ocr=TextRecognition.getClient(TextRecognizerOptions.DEFAULT_OPTIONS)){
            return Tasks.await(ocr.process(InputImage.fromFilePath(ctx,uri))).getText();
        }
    }

    static File speechModel(Context ctx){return new File(ctx.getFilesDir(),"speech-model");}

    static void installSpeech(Context ctx,Uri uri,BooleanSupplier cancelled) throws Exception {
        File target=speechModel(ctx);
        if(target.exists())throw new IOException("Un modèle vocal est déjà installé.");
        File staging=Files.createTempDirectory(ctx.getFilesDir().toPath(),"speech-import").toFile();
        long total=0;int count=0;
        try(ZipInputStream zip=new ZipInputStream(new BufferedInputStream(ctx.getContentResolver().openInputStream(uri)))){
            ZipEntry item;byte[] buf=new byte[65536];
            while((item=zip.getNextEntry())!=null){
                if(++count>2000)throw new IOException("Archive trop volumineuse.");
                File f=new File(staging,item.getName());
                if(!f.getCanonicalPath().startsWith(staging.getCanonicalPath()+File.separator))throw new IOException("Chemin invalide.");
                if(item.isDirectory())continue;
                Files.createDirectories(f.getParentFile().toPath());
                try(OutputStream out=Files.newOutputStream(f.toPath(),StandardOpenOption.CREATE_NEW)){
                    int n;while((n=zip.read(buf))!=-1){
                        total+=n;if(total>512L*1024*1024||cancelled.getAsBoolean())throw new IOException("Import interrompu ou archive trop grande.");
                        out.write(buf,0,n);
                    }
                }
            }
            File folder=staging;
            if(!new File(folder,"am/final.mdl").isFile()){
                File[] dirs=staging.listFiles(File::isDirectory);
                if(dirs==null||dirs.length!=1)throw new IOException("Archive de modèle Vosk attendue.");
                folder=dirs[0];
            }
            try(Model ignored=new Model(folder.getAbsolutePath())){}
            Files.move(folder.toPath(),target.toPath());
        }finally{erase(staging);}
    }
    static boolean erase(File file){
        if(!file.exists())return true;
        File[] children=file.listFiles();
        if(children!=null)for(File f:children)if(!erase(f))return false;
        return file.delete();
    }
    private static void append(StringBuilder text,String json)throws Exception{
        String segment=new JSONObject(json).optString("text").trim();
        if(!segment.isEmpty())text.append(segment).append(' ');
        if(text.length()>6000)throw new IOException("Transcription trop longue pour le modèle mobile.");
    }
    static String audio(Context ctx,Uri uri,BooleanSupplier cancelled)throws Exception{
        File folder=speechModel(ctx);
        if(!folder.isDirectory())throw new IOException("Importez d’abord un modèle vocal Vosk.");
        MediaExtractor source=new MediaExtractor();
        MediaCodec decoder=null;Recognizer recognizer=null;
        Model model=null;
        try{model=new Model(folder.getAbsolutePath());
            source.setDataSource(ctx,uri,null);
            MediaFormat format=null;
            for(int i=0;i<source.getTrackCount();i++){
                MediaFormat track=source.getTrackFormat(i);
                if(track.getString(MediaFormat.KEY_MIME).startsWith("audio/")){format=track;source.selectTrack(i);break;}
            }
            if(format==null)throw new IOException("Piste audio absente.");
            String mime=format.getString(MediaFormat.KEY_MIME);
            decoder=MediaCodec.createDecoderByType(mime);
            decoder.configure(format,null,null,0);decoder.start();
            StringBuilder result=new StringBuilder();
            MediaCodec.BufferInfo info=new MediaCodec.BufferInfo();boolean end=false;
            while(!cancelled.getAsBoolean()){
                if(!end){
                    int index=decoder.dequeueInputBuffer(10000);
                    if(index>=0){
                        int n=source.readSampleData(decoder.getInputBuffer(index),0);
                        long time=source.getSampleTime();
                        if(time>600000000L)throw new IOException("Audio limité à 10 minutes.");
                        end=n<0;
                        decoder.queueInputBuffer(index,0,Math.max(0,n),Math.max(0,time),end?MediaCodec.BUFFER_FLAG_END_OF_STREAM:0);
                        if(!end)source.advance();
                    }
                }
                int index=decoder.dequeueOutputBuffer(info,10000);
                if(index<0)continue;
                if(info.size>0){
                    MediaFormat pcm=decoder.getOutputFormat(index);
                    if(pcm.containsKey(MediaFormat.KEY_PCM_ENCODING)&&pcm.getInteger(MediaFormat.KEY_PCM_ENCODING)!=AudioFormat.ENCODING_PCM_16BIT)throw new IOException("Décodage PCM 16 bits requis.");
                    int channels=pcm.getInteger(MediaFormat.KEY_CHANNEL_COUNT);
                    if(recognizer==null)recognizer=new Recognizer(model,pcm.getInteger(MediaFormat.KEY_SAMPLE_RATE));
                    ByteBuffer data=decoder.getOutputBuffer(index).order(ByteOrder.LITTLE_ENDIAN);
                    data.position(info.offset);data.limit(info.offset+info.size);
                    short[] mono=new short[info.size/(2*channels)];
                    for(int i=0;i<mono.length;i++){
                        int sum=0;for(int c=0;c<channels;c++)sum+=data.getShort();
                        mono[i]=(short)(sum/channels);
                    }
                    if(recognizer.acceptWaveForm(mono,mono.length))append(result,recognizer.getResult());
                }
                decoder.releaseOutputBuffer(index,false);
                if((info.flags&MediaCodec.BUFFER_FLAG_END_OF_STREAM)!=0)break;
            }
            if(cancelled.getAsBoolean())throw new IOException("Transcription annulée.");
            if(recognizer!=null)append(result,recognizer.getFinalResult());
            return result.toString().trim();
        }finally{
            if(recognizer!=null)recognizer.close();
            if(model!=null)model.close();
            if(decoder!=null)decoder.release();
            source.release();
        }
    }
}
